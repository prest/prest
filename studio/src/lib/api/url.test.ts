import { describe, expect, it } from 'vitest'
import {
	buildPath,
	buildQueryString,
	buildSearchParams,
	buildUrl,
	encodeSegment,
	showPath,
	tablePath,
	type QuerySpec,
} from '@/lib/api/url'

describe('encodeSegment', () => {
	it('escapes slashes and reserved characters', () => {
		expect(encodeSegment('a/b')).toBe('a%2Fb')
		expect(encodeSegment('a b')).toBe('a%20b')
	})
})

describe('buildPath', () => {
	it('joins and encodes segments with a leading slash', () => {
		expect(buildPath('db', 'public', 'my table')).toBe('/db/public/my%20table')
	})

	it('drops empty segments and coerces numbers', () => {
		expect(buildPath('a', '', 1)).toBe('/a/1')
	})
})

describe('tablePath / showPath', () => {
	it('builds data and structure paths', () => {
		expect(tablePath('db', 'public', 'users')).toBe('/db/public/users')
		expect(showPath('db', 'public', 'users')).toBe('/show/db/public/users')
	})
})

describe('buildSearchParams', () => {
	it('encodes binary filters as $op.value', () => {
		const spec: QuerySpec = { filters: [{ column: 'name', op: 'eq', value: 'jon' }] }
		expect(buildSearchParams(spec).get('name')).toBe('$eq.jon')
	})

	it('encodes unary filters without a value', () => {
		const spec: QuerySpec = { filters: [{ column: 'deleted_at', op: 'null' }] }
		expect(buildSearchParams(spec).get('deleted_at')).toBe('$null')
	})

	it('skips filters without a column', () => {
		const spec: QuerySpec = { filters: [{ column: '', op: 'eq', value: 'x' }] }
		expect([...buildSearchParams(spec).keys()]).toHaveLength(0)
	})

	it('defaults a missing binary value to empty string', () => {
		const spec: QuerySpec = { filters: [{ column: 'a', op: 'eq' }] }
		expect(buildSearchParams(spec).get('a')).toBe('$eq.')
	})

	it('adds select, order and pagination params', () => {
		const spec: QuerySpec = {
			select: ['id', 'name'],
			order: ['-created_at'],
			page: 2,
			pageSize: 25,
		}
		const params = buildSearchParams(spec)
		expect(params.get('_select')).toBe('id,name')
		expect(params.get('_order')).toBe('-created_at')
		expect(params.get('_page')).toBe('2')
		expect(params.get('_page_size')).toBe('25')
	})

	it('omits pagination when non-positive', () => {
		const params = buildSearchParams({ page: 0, pageSize: -1 })
		expect(params.has('_page')).toBe(false)
		expect(params.has('_page_size')).toBe(false)
	})

	it('merges extra params last', () => {
		const params = buildSearchParams({ extra: { _count: '*' } })
		expect(params.get('_count')).toBe('*')
	})
})

describe('buildQueryString / buildUrl', () => {
	it('produces a query string', () => {
		expect(buildQueryString({ page: 1 })).toBe('_page=1')
	})

	it('returns the bare path when no spec is given', () => {
		expect(buildUrl('/db/public/users')).toBe('/db/public/users')
	})

	it('appends a query string when present', () => {
		expect(buildUrl('/t', { page: 1 })).toBe('/t?_page=1')
	})

	it('returns the bare path when the spec is empty', () => {
		expect(buildUrl('/t', {})).toBe('/t')
	})
})
