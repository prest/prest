import { describe, expect, it, vi } from 'vitest'
import { PrestClient } from '@/lib/api/client'
import { ApiError } from '@/lib/errors'

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
	return new Response(typeof body === 'string' ? body : JSON.stringify(body), {
		status: 200,
		headers: { 'content-type': 'application/json' },
		...init,
	})
}

describe('PrestClient.resolve', () => {
	const client = new PrestClient({ baseUrl: 'http://host/api/' })

	it('prefixes relative paths with the base url and normalizes slashes', () => {
		expect(client.resolve('databases')).toBe('http://host/api/databases')
		expect(client.resolve('/databases')).toBe('http://host/api/databases')
	})

	it('leaves absolute urls untouched', () => {
		expect(client.resolve('https://x/y')).toBe('https://x/y')
	})

	it('appends a query string', () => {
		expect(client.resolve('/t', '_page=1')).toBe('http://host/api/t?_page=1')
		expect(client.resolve('/t', new URLSearchParams({ a: 'b' }))).toBe('http://host/api/t?a=b')
	})

	it('uses & when the path already has a query', () => {
		expect(client.resolve('/t?x=1', 'y=2')).toBe('http://host/api/t?x=1&y=2')
	})

	it('ignores an empty query', () => {
		expect(client.resolve('/t', '')).toBe('http://host/api/t')
	})
})

describe('PrestClient headers', () => {
	it('injects a bearer token when available', async () => {
		const fetchImpl = vi.fn(async (_url: RequestInfo | URL, _init?: RequestInit) =>
			jsonResponse([]),
		)
		const client = new PrestClient({ getToken: () => 'tok', fetchImpl })
		await client.requestJson('/t')
		const headers = (fetchImpl.mock.calls[0][1] as RequestInit).headers as Headers
		expect(headers.get('Authorization')).toBe('Bearer tok')
		expect(headers.get('Accept')).toBe('application/json')
	})

	it('skips auth when noAuth is set', async () => {
		const fetchImpl = vi.fn(async (_url: RequestInfo | URL, _init?: RequestInit) =>
			jsonResponse([]),
		)
		const client = new PrestClient({ getToken: () => 'tok', fetchImpl })
		await client.requestJson('/t', { noAuth: true })
		const headers = (fetchImpl.mock.calls[0][1] as RequestInit).headers as Headers
		expect(headers.has('Authorization')).toBe(false)
	})

	it('sets Content-Type and serializes a JSON body, honoring custom headers', async () => {
		const fetchImpl = vi.fn(async (_url: RequestInfo | URL, _init?: RequestInit) =>
			jsonResponse({ ok: true }),
		)
		const client = new PrestClient({ fetchImpl })
		await client.requestJson('/t', { method: 'POST', body: { a: 1 }, headers: { 'X-Test': '1' } })
		const init = fetchImpl.mock.calls[0][1] as RequestInit
		const headers = init.headers as Headers
		expect(headers.get('Content-Type')).toBe('application/json')
		expect(headers.get('X-Test')).toBe('1')
		expect(init.body).toBe('{"a":1}')
	})
})

describe('PrestClient.requestJson', () => {
	it('parses a JSON array response', async () => {
		const client = new PrestClient({ fetchImpl: async () => jsonResponse([{ id: 1 }]) })
		const res = await client.requestJson<{ id: number }[]>('/t')
		expect(res.data).toEqual([{ id: 1 }])
		expect(res.status).toBe(200)
		expect(res.durationMs).toBeGreaterThanOrEqual(0)
	})

	it('returns undefined data for an empty body', async () => {
		const client = new PrestClient({ fetchImpl: async () => jsonResponse('') })
		const res = await client.requestJson('/t')
		expect(res.data).toBeUndefined()
	})

	it('throws an ApiError with kind and status on non-2xx', async () => {
		const client = new PrestClient({
			fetchImpl: async () => jsonResponse({ error: 'nope' }, { status: 401 }),
		})
		await expect(client.requestJson('/t')).rejects.toMatchObject({
			name: 'ApiError',
			kind: 'unauthorized',
			status: 401,
		})
	})

	it('uses the body error message for unmapped statuses', async () => {
		const client = new PrestClient({
			fetchImpl: async () => jsonResponse({ error: 'conflict' }, { status: 409 }),
		})
		await expect(client.requestJson('/t')).rejects.toThrow('conflict')
	})

	it('falls back to a plain-text error body', async () => {
		const client = new PrestClient({
			fetchImpl: async () => new Response('bad things', { status: 409 }),
		})
		await expect(client.requestJson('/t')).rejects.toThrow('bad things')
	})

	it('throws a parse ApiError on invalid JSON', async () => {
		const client = new PrestClient({
			fetchImpl: async () => new Response('not json', { status: 200 }),
		})
		await expect(client.requestJson('/t')).rejects.toMatchObject({ kind: 'parse' })
	})
})

describe('PrestClient.probe / requestRaw', () => {
	it('probe reports status without reading the body', async () => {
		const client = new PrestClient({ fetchImpl: async () => new Response('', { status: 200 }) })
		expect(await client.probe('/_health')).toEqual({ status: 200, ok: true })
	})

	it('requestRaw returns text and metadata', async () => {
		const client = new PrestClient({
			fetchImpl: async () => new Response('hello', { status: 200 }),
		})
		const res = await client.requestRaw('/t')
		expect(res.text).toBe('hello')
		expect(res.ok).toBe(true)
		expect(res.status).toBe(200)
	})
})

describe('PrestClient failure handling', () => {
	it('wraps network failures as ApiError(network)', async () => {
		const client = new PrestClient({
			fetchImpl: async () => {
				throw new TypeError('offline')
			},
		})
		await expect(client.probe('/t')).rejects.toMatchObject({ kind: 'network' })
	})

	it('reports a timeout as ApiError(timeout)', async () => {
		const client = new PrestClient({
			timeoutMs: 5,
			fetchImpl: (_url, init) =>
				new Promise((_resolve, reject) => {
					init?.signal?.addEventListener('abort', () =>
						reject(new DOMException('aborted', 'AbortError')),
					)
				}),
		})
		const err = await client.probe('/t').catch((e) => e)
		expect(err).toBeInstanceOf(ApiError)
		expect(err.kind).toBe('timeout')
	})

	it('propagates an external abort signal', async () => {
		const controller = new AbortController()
		controller.abort()
		const client = new PrestClient({
			fetchImpl: (_url, init) =>
				new Promise((_resolve, reject) => {
					const signal = init?.signal
					if (signal?.aborted) {
						reject(new DOMException('aborted', 'AbortError'))
						return
					}
					signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
				}),
		})
		await expect(client.probe('/t', { signal: controller.signal })).rejects.toBeInstanceOf(ApiError)
	})

	it('accepts a live (non-aborted) external signal', async () => {
		const controller = new AbortController()
		const client = new PrestClient({ fetchImpl: async () => jsonResponse([]) })
		const res = await client.requestJson('/t', { signal: controller.signal })
		expect(res.status).toBe(200)
	})
})
