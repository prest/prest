import { describe, expect, it } from 'vitest'
import { PrestClient } from '@/lib/api/client'
import {
	NAME_KEYS,
	getHealth,
	getMeta,
	getReady,
	listDatabases,
	listSchemas,
	listTables,
	pickName,
	probeMcp,
	queryTable,
	showTable,
} from '@/lib/api/prest'

function clientReturning(handler: (url: string) => Response): PrestClient {
	return new PrestClient({ fetchImpl: async (input) => handler(String(input)) })
}

function json(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' },
	})
}

describe('meta and probes', () => {
	it('getMeta requests the studio meta endpoint without auth', async () => {
		const meta = {
			version: '2.0',
			commit: 'abc',
			buildDate: '2026',
			apiBasePath: '/',
			mcpEndpoint: '/_mcp',
		}
		const client = clientReturning((url) => {
			expect(url).toContain('/_studio/api/meta')
			return json(meta)
		})
		expect(await getMeta(client)).toEqual(meta)
	})

	it('probes health, ready and mcp for status only', async () => {
		const client = clientReturning(() => new Response('', { status: 200 }))
		expect(await getHealth(client)).toEqual({ status: 200, ok: true })
		expect(await getReady(client)).toEqual({ status: 200, ok: true })
		expect(await probeMcp(client)).toEqual({ status: 200, ok: true })
	})
})

describe('discovery endpoints', () => {
	it('returns items on success', async () => {
		const client = clientReturning(() => json([{ datname: 'prest' }]))
		const res = await listDatabases(client)
		expect(res.disabled).toBe(false)
		expect(res.items).toHaveLength(1)
	})

	it('normalizes a non-array payload to an empty list', async () => {
		const client = clientReturning(() => json({ not: 'an array' }))
		const res = await listSchemas(client)
		expect(res.items).toEqual([])
	})

	it('treats 403/404 as feature-disabled', async () => {
		const forbidden = clientReturning(() => json({ error: 'no' }, 403))
		expect(await listTables(forbidden)).toEqual({ disabled: true })
		const missing = clientReturning(() => json({ error: 'no' }, 404))
		expect(await listDatabases(missing)).toEqual({ disabled: true })
	})

	it('reports unexpected errors without disabling', async () => {
		const client = clientReturning(() => json({ error: 'boom' }, 500))
		const res = await listSchemas(client)
		expect(res.disabled).toBe(false)
		expect(res.error).toMatch(/Server error/)
	})
})

describe('showTable', () => {
	it('returns the column list', async () => {
		const client = clientReturning((url) => {
			expect(url).toContain('/show/db/public/users')
			return json([{ column_name: 'id', data_type: 'integer', is_nullable: 'NO' }])
		})
		const cols = await showTable(client, 'db', 'public', 'users')
		expect(cols[0].column_name).toBe('id')
	})

	it('returns an empty array for a non-array payload', async () => {
		const client = clientReturning(() => json({}))
		expect(await showTable(client, 'db', 'public', 'users')).toEqual([])
	})
})

describe('queryTable', () => {
	it('builds the data path with query params and returns rows', async () => {
		const client = clientReturning((url) => {
			expect(url).toContain('/db/public/users')
			expect(url).toContain('_page=2')
			expect(url).toContain('_page_size=25')
			return json([{ id: 1 }, { id: 2 }])
		})
		const res = await queryTable(client, 'db', 'public', 'users', { page: 2, pageSize: 25 })
		expect(res.rows).toHaveLength(2)
		expect(res.status).toBe(200)
	})

	it('normalizes a non-array payload to empty rows', async () => {
		const client = clientReturning(() => json({}))
		const res = await queryTable(client, 'db', 'public', 'users', {})
		expect(res.rows).toEqual([])
	})
})

describe('pickName', () => {
	it('prefers candidate keys in order', () => {
		expect(pickName({ datname: 'a', name: 'b' }, [...NAME_KEYS.database])).toBe('a')
	})

	it('falls back to the first string value', () => {
		expect(pickName({ weird: 'value' }, ['nope'])).toBe('value')
	})

	it('returns empty string when no string values exist', () => {
		expect(pickName({ n: 1 }, ['x'])).toBe('')
	})
})
