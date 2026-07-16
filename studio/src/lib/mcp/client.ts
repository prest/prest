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
	id?: number
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
	private sessionId: string | null = null

	constructor(client: PrestClient, options: McpClientOptions = {}) {
		this.client = client
		this.endpoint = options.endpoint ?? '/_mcp'
	}

	/** Perform the MCP `initialize` handshake, then send `notifications/initialized`. */
	async initialize(signal?: AbortSignal): Promise<InitializeResult> {
		const { result, headers } = await this.callWithHeaders<InitializeResult>(
			'initialize',
			{
				protocolVersion: MCP_PROTOCOL_VERSION,
				capabilities: {},
				clientInfo: { name: 'prest-studio', version: '0.1.0' },
			},
			signal,
		)
		const session = headers.get('Mcp-Session-Id')
		this.sessionId = session && session.length > 0 ? session : null
		await this.notify('notifications/initialized', {}, signal)
		return result
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

	private sessionHeaders(): Record<string, string> {
		const headers: Record<string, string> = {
			Accept: 'application/json, text/event-stream',
		}
		if (this.sessionId) headers['Mcp-Session-Id'] = this.sessionId
		return headers
	}

	/** JSON-RPC notification (no id); best-effort, ignores empty bodies. */
	private async notify(method: string, params: unknown, signal?: AbortSignal): Promise<void> {
		const request: JsonRpcRequest = { jsonrpc: '2.0', method, params }
		try {
			await this.client.requestRaw(this.endpoint, {
				method: 'POST',
				body: request,
				headers: this.sessionHeaders(),
				signal,
			})
		} catch (err) {
			throw new McpError(toErrorMessage(err), { kind: 'transport', cause: err })
		}
	}

	private async call<T>(method: string, params: unknown, signal?: AbortSignal): Promise<T> {
		const { result } = await this.callWithHeaders<T>(method, params, signal)
		return result
	}

	/** Send a single JSON-RPC request and return its `result` plus response headers. */
	private async callWithHeaders<T>(
		method: string,
		params: unknown,
		signal?: AbortSignal,
	): Promise<{ result: T; headers: Headers }> {
		const id = ++this.id
		const request: JsonRpcRequest = { jsonrpc: '2.0', id, method, params }

		let raw: { text: string; status: number; ok: boolean; headers: Headers }
		try {
			raw = await this.client.requestRaw(this.endpoint, {
				method: 'POST',
				body: request,
				headers: this.sessionHeaders(),
				signal,
			})
		} catch (err) {
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

		const message = parseRpcBody<T>(raw.text, id)
		if (message.error) {
			throw new McpError(message.error.message || 'MCP protocol error.', {
				kind: 'protocol',
				code: message.error.code,
				data: message.error.data,
			})
		}
		return { result: message.result as T, headers: raw.headers }
	}
}

/**
 * Parse a JSON-RPC response body, transparently unwrapping Streamable-HTTP SSE
 * framing (`event: message\n` / `data: {…}` lines) when present.
 * When `requestId` is provided, selects the message matching that id.
 */
export function parseRpcBody<T>(text: string, requestId?: number): JsonRpcResponse<T> {
	const trimmed = text.trim()
	if (!trimmed) {
		throw new McpError('Empty MCP response body.', { kind: 'parse' })
	}

	if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
		let parsed: unknown
		try {
			parsed = JSON.parse(trimmed)
		} catch (err) {
			throw new McpError('Failed to parse MCP response.', { kind: 'parse', cause: err })
		}
		return selectRpcMessage(parsed, requestId)
	}

	const messages = extractSseMessages(trimmed)
	if (messages.length === 0) {
		throw new McpError('No JSON payload found in MCP SSE response.', { kind: 'parse' })
	}
	return selectRpcMessage(messages, requestId)
}

function selectRpcMessage<T>(parsed: unknown, requestId?: number): JsonRpcResponse<T> {
	const candidates: unknown[] = Array.isArray(parsed) ? parsed : [parsed]
	let message: unknown
	if (requestId !== undefined) {
		message = candidates.find(
			(m) => m && typeof m === 'object' && (m as JsonRpcResponse).id === requestId,
		)
		if (!message && candidates.length === 1) message = candidates[0]
	} else {
		message = candidates[candidates.length - 1]
	}
	if (!message || typeof message !== 'object' || (message as JsonRpcResponse).jsonrpc !== '2.0') {
		throw new McpError('Malformed JSON-RPC response.', { kind: 'parse' })
	}
	return message as JsonRpcResponse<T>
}

/** Parse each SSE event's `data:` payload independently into JSON values. */
function extractSseMessages(text: string): unknown[] {
	const messages: unknown[] = []
	let dataLines: string[] = []

	const flush = () => {
		if (dataLines.length === 0) return
		const payload = dataLines.join('\n')
		dataLines = []
		try {
			messages.push(JSON.parse(payload))
		} catch (err) {
			throw new McpError('Failed to parse MCP response.', { kind: 'parse', cause: err })
		}
	}

	for (const line of text.split(/\r?\n/)) {
		if (line.startsWith('data:')) {
			dataLines.push(line.slice(5).trimStart())
		} else if (line === '') {
			flush()
		}
	}
	flush()
	return messages
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
