import { describe, expect, it } from 'vitest'
import {
	countCatalog,
	groupBySchema,
	isBrowsableRelation,
	isSystemSchema,
	orderColumns,
} from './relations'

describe('isSystemSchema', () => {
	it.each(['pg_catalog', 'information_schema', 'pg_toast', 'pg_temp_3', 'pg_toast_temp_12'])(
		'%s is system',
		(n) => expect(isSystemSchema(n)).toBe(true),
	)
	it.each(['public', 'shop', 'pg_custom', 'my_pg_temp_1'])('%s is not system', (n) =>
		expect(isSystemSchema(n)).toBe(false),
	)
})

describe('isBrowsableRelation', () => {
	it('keeps tables, views, materialized views and missing type', () => {
		expect(isBrowsableRelation({ type: 'table' })).toBe(true)
		expect(isBrowsableRelation({ type: 'view' })).toBe(true)
		expect(isBrowsableRelation({ type: 'materialized view' })).toBe(true)
		expect(isBrowsableRelation({ type: 'MATERIALIZED VIEW' })).toBe(true)
		expect(isBrowsableRelation({ name: 'x' })).toBe(true)
		expect(isBrowsableRelation({ type: '' })).toBe(true)
		expect(isBrowsableRelation({ type: 7 })).toBe(true)
	})
	it('drops sequences and indexes', () => {
		expect(isBrowsableRelation({ type: 'sequence' })).toBe(false)
		expect(isBrowsableRelation({ type: 'Sequence' })).toBe(false)
		expect(isBrowsableRelation({ type: 'index' })).toBe(false)
	})
})

describe('groupBySchema', () => {
	it('filters non-browsable and system schemas and sorts', () => {
		const g = groupBySchema([
			{ name: 'b', schema: 'public', type: 'table' },
			{ name: 'a', schema: 'public', type: 'view' },
			{ name: 'items_id_seq', schema: 'public', type: 'sequence' },
			{ name: 'pg_class', schema: 'pg_catalog', type: 'table' },
			{ name: 't', schema: 'pg_temp_3', type: 'table' },
			{ name: 'orders', schema: 'shop', type: 'table' },
		])
		expect([...g.keys()]).toEqual(['public', 'shop'])
		expect(g.get('public')).toEqual(['a', 'b'])
		expect(g.get('shop')).toEqual(['orders'])
	})
})

describe('groupBySchema ordering', () => {
	it('sorts schemas regardless of input order', () => {
		const g = groupBySchema([
			{ name: 'z', schema: 'zeta', type: 'table' },
			{ name: 'a', schema: 'alpha', type: 'table' },
			{ name: 'b', schema: 'alpha', type: 'table' },
			{ name: 'm', schema: 'mid', type: 'table' },
		])
		expect([...g.keys()]).toEqual(['alpha', 'mid', 'zeta'])
	})
})

describe('orderColumns', () => {
	it('orders by position', () => {
		const s = [
			{ column_name: 'b', position: 2 },
			{ column_name: 'a', position: 1 },
		]
		expect(orderColumns(['a', 'b'], s)).toEqual(['a', 'b'])
		expect(orderColumns(['b', 'a'], s)).toEqual(['a', 'b'])
	})
	it('uses structure order when position missing', () => {
		const s = [{ column_name: 'z' }, { column_name: 'a' }]
		expect(orderColumns(['a', 'z'], s)).toEqual(['z', 'a'])
	})
	it('falls back to row keys without structure', () => {
		expect(orderColumns(['b', 'a'], undefined)).toEqual(['b', 'a'])
		expect(orderColumns(['b', 'a'], [])).toEqual(['b', 'a'])
	})
	it('appends unknown keys in first-seen order', () => {
		const s = [{ column_name: 'a', position: 1 }]
		expect(orderColumns(['x', 'a', 'y'], s)).toEqual(['a', 'x', 'y'])
	})
	it('drops structure columns absent from rows and returns [] for empty rows', () => {
		const s = [{ column_name: 'a' }, { column_name: 'b' }]
		expect(orderColumns(['b'], s)).toEqual(['b'])
		expect(orderColumns([], s)).toEqual([])
	})
	it('ignores structure entries without a name', () => {
		expect(orderColumns(['a'], [{ position: 1 }, { column_name: 'a' }])).toEqual(['a'])
	})
})

describe('countCatalog', () => {
	it('counts non-system schemas and browsable relations (main fixture)', () => {
		const schemas = [
			{ schema_name: 'public' },
			{ schema_name: 'shop' },
			{ schema_name: 'pg_catalog' },
			{ schema_name: 'information_schema' },
			{ schema_name: 'pg_toast' },
		]
		const tables = [
			{ name: 'items', schema: 'public', type: 'table' },
			{ name: 'active_items', schema: 'public', type: 'view' },
			{ name: 'Mixed Case', schema: 'public', type: 'table' },
			{ name: 'TestCase', schema: 'public', type: 'table' },
			{ name: 'big', schema: 'public', type: 'table' },
			{ name: 'orders', schema: 'shop', type: 'table' },
			{ name: 'items_id_seq', schema: 'public', type: 'sequence' },
			{ name: 'orders_id_seq', schema: 'shop', type: 'sequence' },
			{ name: 'pg_class', schema: 'pg_catalog', type: 'table' },
		]
		expect(countCatalog(schemas, tables)).toEqual({ schemas: 2, tables: 6 })
	})
	it('returns undefined counts when inputs are undefined', () => {
		expect(countCatalog(undefined, undefined)).toEqual({ schemas: undefined, tables: undefined })
	})
})
