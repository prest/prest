/**
 * Minimal JSON-RPC 2.0 client for pREST's Model Context Protocol endpoint
 * (`/_mcp`).
 *
 * Design constraints:
 *  - Speaks JSON-RPC 2.0 over HTTP POST.
 *  - Understands both plain `application/json` responses and the
 *    Streamable-HTTP SSE framing (`text/event-stream`) MCP servers may emit.
 *  - Surfaces failures as typed {@link McpError}s (transport vs protocol).
 *  - Performs NO automatic retries – a failed call is reported to the caller.
 */

import type { PrestClient } from '@/lib/api/client'
import { ApiError, toErrorMessage } from '@/lib/errors'

export const MCP_PROTOCOL_VERSION = '2025-06-18'

export type McpErrorKind = 'transport' | 'protocol' | 'parse'

export class McpError extends Error {
	readonly kind: McpErrorKind
	/** JSON-RPC error code when {@link kind} is `protocol`. */
	readonly code?: number
	readonly data?: unknown

	constructor(
		message: string,
		options: { kind: McpErrorKind; code?: number; data?: unknown; cause?: unknown },
	) {
		super(message)
		this.name = 'McpError'
		this.kind = options.kind
		this.code = options.code
		this.data = options.data
		if (options.cause !== undefined) {
			;(this as { cause?: unknown }).cause = options.cause
		}
	}
}

export interface JsonRpcRequest {
	jsonrpc: '2.0'
	id: number
	method: string
	params?: unknown
}

export interface JsonRpcError {
	code: number
	message: string
	data?: unknown
}

export interface JsonRpcResponse<T = unknown> {
	jsonrpc: '2.0'
	id: number | null
	result?: T
	error?: JsonRpcError
}

export interface McpToolAnnotations {
	title?: string
	readOnlyHint?: boolean
	destructiveHint?: boolean
	idempotentHint?: boolean
	openWorldHint?: boolean
}

export interface McpTool {
	name: string
	description?: string
	inputSchema?: Record<string, unknown>
	annotations?: McpToolAnnotations
}

export interface McpServerInfo {
	name?: string
	version?: string
	[key: string]: unknown
}

export interface InitializeResult {
	protocolVersion?: string
	capabilities?: Record<string, unknown>
	serverInfo?: McpServerInfo
	instructions?: string
	[key: string]: unknown
}

export interface ListToolsResult {
	tools: McpTool[]
	nextCursor?: string
}

export interface McpContentBlock {
	type: string
	text?: string
	[key: string]: unknown
}

export interface CallToolResult {
	content?: McpContentBlock[]
	structuredContent?: unknown
	isError?: boolean
	[key: string]: unknown
}

export interface McpClientOptions {
	/** Endpoint path for the MCP server. Defaults to `/_mcp`. */
	endpoint?: string
}

export class McpClient {
	private readonly client: PrestClient
	private readonly endpoint: string
	private id = 0

	constructor(client: PrestClient, options: McpClientOptions = {}) {
		this.client = client
		this.endpoint = options.endpoint ?? '/_mcp'
	}

	/** Perform the MCP `initialize` handshake. */
	async initialize(signal?: AbortSignal): Promise<InitializeResult> {
		return this.call<InitializeResult>(
			'initialize',
			{
				protocolVersion: MCP_PROTOCOL_VERSION,
				capabilities: {},
				clientInfo: { name: 'prest-studio', version: '0.1.0' },
			},
			signal,
		)
	}

	/** List available tools (`tools/list`). */
	async listTools(cursor?: string, signal?: AbortSignal): Promise<ListToolsResult> {
		const result = await this.call<ListToolsResult>('tools/list', cursor ? { cursor } : {}, signal)
		return {
			tools: Array.isArray(result?.tools) ? result.tools : [],
			nextCursor: result?.nextCursor,
		}
	}

	/** Invoke a tool (`tools/call`). */
	async callTool(
		name: string,
		args: Record<string, unknown> = {},
		signal?: AbortSignal,
	): Promise<CallToolResult> {
		return this.call<CallToolResult>('tools/call', { name, arguments: args }, signal)
	}

	/** Send a single JSON-RPC request and return its `result`, or throw. */
	private async call<T>(method: string, params: unknown, signal?: AbortSignal): Promise<T> {
		const id = ++this.id
		const request: JsonRpcRequest = { jsonrpc: '2.0', id, method, params }

		let raw: { text: string; status: number; ok: boolean }
		try {
			raw = await this.client.requestRaw(this.endpoint, {
				method: 'POST',
				body: request,
				headers: { Accept: 'application/json, text/event-stream' },
				signal,
			})
		} catch (err) {
			// PrestClient already normalized network/timeout failures.
			throw new McpError(toErrorMessage(err), {
				kind: 'transport',
				cause: err,
			})
		}

		if (!raw.ok) {
			throw new McpError(`MCP transport error (HTTP ${raw.status}).`, {
				kind: 'transport',
				code: raw.status,
			})
		}

		const message = parseRpcBody<T>(raw.text)
		if (message.error) {
			throw new McpError(message.error.message || 'MCP protocol error.', {
				kind: 'protocol',
				code: message.error.code,
				data: message.error.data,
			})
		}
		return message.result as T
	}
}

/**
 * Parse a JSON-RPC response body, transparently unwrapping Streamable-HTTP SSE
 * framing (`event: message\n` / `data: {…}` lines) when present.
 */
export function parseRpcBody<T>(text: string): JsonRpcResponse<T> {
	const trimmed = text.trim()
	if (!trimmed) {
		throw new McpError('Empty MCP response body.', { kind: 'parse' })
	}

	const payload =
		trimmed.startsWith('{') || trimmed.startsWith('[') ? trimmed : extractSseData(trimmed)

	let parsed: unknown
	try {
		parsed = JSON.parse(payload)
	} catch (err) {
		throw new McpError('Failed to parse MCP response.', { kind: 'parse', cause: err })
	}

	// A batch response is not expected for single requests; take the last frame.
	const message = Array.isArray(parsed) ? parsed[parsed.length - 1] : parsed
	if (!message || typeof message !== 'object' || (message as JsonRpcResponse).jsonrpc !== '2.0') {
		throw new McpError('Malformed JSON-RPC response.', { kind: 'parse' })
	}
	return message as JsonRpcResponse<T>
}

/** Pull the concatenated `data:` payload out of an SSE stream chunk. */
function extractSseData(text: string): string {
	const dataLines: string[] = []
	for (const line of text.split(/\r?\n/)) {
		if (line.startsWith('data:')) {
			dataLines.push(line.slice(5).trimStart())
		}
	}
	if (dataLines.length === 0) {
		throw new McpError('No JSON payload found in MCP SSE response.', { kind: 'parse' })
	}
	return dataLines.join('')
}

/** Flatten a tool result's content blocks into a display string. */
export function contentToText(result: CallToolResult): string {
	if (result.structuredContent !== undefined) {
		return JSON.stringify(result.structuredContent, null, 2)
	}
	const blocks = result.content ?? []
	const texts = blocks
		.map((b) => (typeof b.text === 'string' ? b.text : JSON.stringify(b, null, 2)))
		.filter((s) => s.length > 0)
	return texts.join('\n')
}

/** Convenience re-export so callers can narrow transport errors uniformly. */
export { ApiError }

/** Group tools into catalog vs per-table based on pREST naming. */
export function groupTools(tools: McpTool[]): {
	catalog: McpTool[]
	table: McpTool[]
	other: McpTool[]
} {
	const catalog: McpTool[] = []
	const table: McpTool[] = []
	const other: McpTool[] = []
	for (const t of tools) {
		if (
			t.name === 'prest.list_databases' ||
			t.name === 'prest.list_schemas' ||
			t.name === 'prest.list_tables' ||
			t.name === 'prest.describe_table' ||
			t.name === 'prest.select_table'
		) {
			catalog.push(t)
		} else if (t.name.startsWith('prest.select.')) {
			table.push(t)
		} else {
			other.push(t)
		}
	}
	return { catalog, table, other }
}
