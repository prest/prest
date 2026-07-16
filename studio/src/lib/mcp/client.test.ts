import { describe, expect, it, vi } from 'vitest'
import { PrestClient } from '@/lib/api/client'
import {
	McpClient,
	McpError,
	contentToText,
	parseRpcBody,
	type CallToolResult,
} from '@/lib/mcp/client'

function mcpClient(handler: (body: unknown) => Response): {
	client: McpClient
	fetchImpl: ReturnType<typeof vi.fn>
} {
	const fetchImpl = vi.fn(async (_url: unknown, init?: RequestInit) => {
		const body = init?.body ? JSON.parse(String(init.body)) : undefined
		return handler(body)
	})
	const prest = new PrestClient({ fetchImpl: fetchImpl as unknown as typeof fetch })
	return { client: new McpClient(prest), fetchImpl }
}

function rpc(result: unknown, id = 1): Response {
	return new Response(JSON.stringify({ jsonrpc: '2.0', id, result }), {
		status: 200,
		headers: { 'content-type': 'application/json' },
	})
}

describe('McpClient.initialize', () => {
	it('sends a JSON-RPC initialize request and returns the result', async () => {
		const { client, fetchImpl } = mcpClient(() =>
			rpc({ protocolVersion: '2025-06-18', serverInfo: { name: 'prest', version: '2' } }),
		)
		const info = await client.initialize()
		expect(info.serverInfo?.name).toBe('prest')

		const sent = JSON.parse(String((fetchImpl.mock.calls[0][1] as RequestInit).body))
		expect(sent.jsonrpc).toBe('2.0')
		expect(sent.method).toBe('initialize')
		expect(sent.id).toBe(1)
		const headers = (fetchImpl.mock.calls[0][1] as RequestInit).headers as Headers
		expect(headers.get('Accept')).toContain('text/event-stream')
	})
})

describe('McpClient.listTools', () => {
	it('returns the tools array', async () => {
		const { client } = mcpClient(() => rpc({ tools: [{ name: 'query' }] }))
		const res = await client.listTools()
		expect(res.tools).toEqual([{ name: 'query' }])
	})

	it('passes a cursor and defaults missing tools to an empty array', async () => {
		const { client, fetchImpl } = mcpClient(() => rpc({}))
		const res = await client.listTools('next')
		expect(res.tools).toEqual([])
		const sent = JSON.parse(String((fetchImpl.mock.calls[0][1] as RequestInit).body))
		expect(sent.params).toEqual({ cursor: 'next' })
	})
})

describe('McpClient.callTool', () => {
	it('sends the tool name and arguments', async () => {
		const { client, fetchImpl } = mcpClient(() => rpc({ content: [{ type: 'text', text: 'ok' }] }))
		const res = await client.callTool('query', { sql: 'select 1' })
		expect(res.content?.[0].text).toBe('ok')
		const sent = JSON.parse(String((fetchImpl.mock.calls[0][1] as RequestInit).body))
		expect(sent.method).toBe('tools/call')
		expect(sent.params).toEqual({ name: 'query', arguments: { sql: 'select 1' } })
	})

	it('defaults arguments to an empty object', async () => {
		const { client, fetchImpl } = mcpClient(() => rpc({}))
		await client.callTool('ping')
		const sent = JSON.parse(String((fetchImpl.mock.calls[0][1] as RequestInit).body))
		expect(sent.params).toEqual({ name: 'ping', arguments: {} })
	})
})

describe('McpClient error handling', () => {
	it('raises a protocol McpError from a JSON-RPC error', async () => {
		const { client } = mcpClient(
			() =>
				new Response(
					JSON.stringify({ jsonrpc: '2.0', id: 1, error: { code: -32601, message: 'no method' } }),
					{ status: 200 },
				),
		)
		const err = await client.initialize().catch((e) => e)
		expect(err).toBeInstanceOf(McpError)
		expect(err.kind).toBe('protocol')
		expect(err.code).toBe(-32601)
	})

	it('raises a transport McpError for a non-2xx response', async () => {
		const { client } = mcpClient(() => new Response('nope', { status: 500 }))
		const err = await client.initialize().catch((e) => e)
		expect(err).toBeInstanceOf(McpError)
		expect(err.kind).toBe('transport')
		expect(err.code).toBe(500)
	})

	it('wraps a network failure as a transport McpError', async () => {
		const prest = new PrestClient({
			fetchImpl: async () => {
				throw new TypeError('offline')
			},
		})
		const client = new McpClient(prest)
		const err = await client.initialize().catch((e) => e)
		expect(err).toBeInstanceOf(McpError)
		expect(err.kind).toBe('transport')
	})

	it('preserves McpError cause metadata', () => {
		const cause = new Error('x')
		const err = new McpError('m', { kind: 'parse', cause })
		expect((err as { cause?: unknown }).cause).toBe(cause)
	})
})

describe('parseRpcBody', () => {
	it('parses a plain JSON body', () => {
		const msg = parseRpcBody('{"jsonrpc":"2.0","id":1,"result":{"a":1}}')
		expect(msg.result).toEqual({ a: 1 })
	})

	it('unwraps Streamable-HTTP SSE framing', () => {
		const sse = 'event: message\ndata: {"jsonrpc":"2.0","id":1,"result":{"ok":true}}\n\n'
		expect(parseRpcBody(sse).result).toEqual({ ok: true })
	})

	it('takes the last frame of a batch response', () => {
		const batch = '[{"jsonrpc":"2.0","id":0,"result":1},{"jsonrpc":"2.0","id":1,"result":2}]'
		expect(parseRpcBody(batch).result).toBe(2)
	})

	it('throws on an empty body', () => {
		expect(() => parseRpcBody('   ')).toThrow(McpError)
	})

	it('throws on invalid JSON', () => {
		expect(() => parseRpcBody('data: not json\n')).toThrow(McpError)
	})

	it('throws when SSE framing has no data lines', () => {
		expect(() => parseRpcBody('event: ping\n')).toThrow(/No JSON payload/)
	})

	it('throws on a non JSON-RPC object', () => {
		expect(() => parseRpcBody('{"foo":"bar"}')).toThrow(/Malformed JSON-RPC/)
	})
})

describe('contentToText', () => {
	it('serializes structured content when present', () => {
		const result: CallToolResult = { structuredContent: { rows: [1, 2] } }
		expect(contentToText(result)).toContain('"rows"')
	})

	it('joins text blocks', () => {
		const result: CallToolResult = {
			content: [
				{ type: 'text', text: 'line1' },
				{ type: 'text', text: 'line2' },
			],
		}
		expect(contentToText(result)).toBe('line1\nline2')
	})

	it('serializes non-text blocks', () => {
		const result: CallToolResult = { content: [{ type: 'image', data: 'x' }] }
		expect(contentToText(result)).toContain('"image"')
	})

	it('returns an empty string when there is no content', () => {
		expect(contentToText({})).toBe('')
	})
})
