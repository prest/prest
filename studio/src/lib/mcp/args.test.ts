import { describe, expect, it, vi } from 'vitest'
import { buildFormArgs, coerceArg } from '@/lib/mcp/args'

describe('coerceArg', () => {
	it('returns strings unchanged for string/unknown types', () => {
		expect(coerceArg('a', 'hello', 'string')).toBe('hello')
		expect(coerceArg('a', 'hello')).toBe('hello')
	})

	it('parses numbers and integers', () => {
		expect(coerceArg('n', '3', 'number')).toBe(3)
		expect(coerceArg('n', '2.5', 'number')).toBe(2.5)
		expect(coerceArg('n', '7', 'integer')).toBe(7)
	})

	it('rejects non-numeric number input', () => {
		expect(() => coerceArg('limit', 'abc', 'number')).toThrow('Field "limit" must be a number.')
		expect(() => coerceArg('limit', 'x', 'integer')).toThrow('Field "limit" must be an integer.')
		expect(() => coerceArg('limit', 'Infinity', 'number')).toThrow(
			'Field "limit" must be a number.',
		)
	})

	it('accepts only true/false for booleans', () => {
		expect(coerceArg('b', 'true', 'boolean')).toBe(true)
		expect(coerceArg('b', 'false', 'boolean')).toBe(false)
		expect(() => coerceArg('b', 'yes', 'boolean')).toThrow('Field "b" must be true or false.')
	})

	it('parses JSON arrays when input starts with [', () => {
		expect(coerceArg('c', '["id","name"]', 'array')).toEqual(['id', 'name'])
		expect(coerceArg('c', ' [1, 2]', 'array')).toEqual([1, 2])
	})

	it('rejects [ input that is not a JSON array', () => {
		const msg = 'Field "c" must be a JSON array.'
		expect(() => coerceArg('c', '[bad', 'array')).toThrow(msg)
		expect(() => coerceArg('c', '[1,', 'array')).toThrow(msg)
	})

	it('rejects non-integers for integer fields', () => {
		for (const v of ['1.5', '0x10', '1e400', 'abc']) {
			expect(() => coerceArg('n', v, 'integer')).toThrow('Field "n" must be an integer.')
		}
		expect(coerceArg('n', ' -4 ', 'integer')).toBe(-4)
	})

	it('rejects hex and non-finite numbers but keeps decimals', () => {
		expect(() => coerceArg('n', '0x10', 'number')).toThrow('Field "n" must be a number.')
		expect(() => coerceArg('n', '1e400', 'number')).toThrow('Field "n" must be a number.')
		expect(coerceArg('n', '1e3', 'number')).toBe(1000)
		expect(coerceArg('n', '.5', 'number')).toBe(0.5)
	})

	it('comma-splits arrays, trimming and dropping empties', () => {
		expect(coerceArg('c', 'id,name', 'array')).toEqual(['id', 'name'])
		expect(coerceArg('c', ' id , , name ,', 'array')).toEqual(['id', 'name'])
	})

	it('parses plain JSON objects', () => {
		expect(coerceArg('f', '{"id": 3}', 'object')).toEqual({ id: 3 })
	})

	it('rejects non-object values for object fields', () => {
		const msg = 'Field "filters" must be a JSON object.'
		expect(() => coerceArg('filters', '[1]', 'object')).toThrow(msg)
		expect(() => coerceArg('filters', 'null', 'object')).toThrow(msg)
		expect(() => coerceArg('filters', '{bad', 'object')).toThrow(msg)
	})

	it('rejects integers that cannot be sent exactly, including nested ones', () => {
		const msg =
			'Field "filters" has an integer literal outside ±9007199254740991 that would lose precision; send it as a string.'
		expect(() => coerceArg('filters', '{"id": 9007199254740993}', 'object')).toThrow(msg)
		expect(() => coerceArg('filters', '{"a": {"b": [-9007199254740993]}}', 'object')).toThrow(msg)
	})

	it('keeps safe integers, decimals and exponent literals in objects', () => {
		expect(coerceArg('f', '{"id": 9007199254740991, "p": 1.5, "e": 1e300}', 'object')).toEqual({
			id: 9007199254740991,
			p: 1.5,
			e: 1e300,
		})
	})

	it('rejects unsafe integers in JSON array fields too', () => {
		expect(() => coerceArg('ids', '[1, 9007199254740993]', 'array')).toThrow(
			'Field "ids" has an integer literal outside ±9007199254740991 that would lose precision; send it as a string.',
		)
	})
})

describe('buildFormArgs', () => {
	const props: [string, { type?: string }][] = [
		['table', { type: 'string' }],
		['columns', { type: 'array' }],
		['limit', { type: 'integer' }],
	]

	it('coerces filled values and skips empty ones', () => {
		expect(
			buildFormArgs(props, new Set(['table']), { table: 'items', columns: 'id,name', limit: '' }),
		).toEqual({
			table: 'items',
			columns: ['id', 'name'],
		})
	})

	it('treats whitespace-only values as empty', () => {
		expect(
			buildFormArgs(props, new Set(['table']), { table: 'items', columns: '  ', limit: ' ' }),
		).toEqual({
			table: 'items',
		})
		expect(() => buildFormArgs(props, new Set(['columns']), { columns: '   ' })).toThrow(
			'Missing required fields: columns',
		)
	})

	it('throws for missing required fields', () => {
		expect(() => buildFormArgs(props, new Set(['table', 'limit']), { columns: 'id' })).toThrow(
			'Missing required fields: table, limit',
		)
	})

	it('propagates coercion errors', () => {
		expect(() => buildFormArgs(props, new Set(), { limit: 'x' })).toThrow(
			'Field "limit" must be an integer.',
		)
	})
})

// JSON.parse is stubbed globally here, so this block must not run concurrently with others.
describe('coerceArg without JSON.parse source access', { concurrent: false }, () => {
	it('accepts exponent and decimal values and rejects only unsafe integer literals', () => {
		// Scenario: older runtimes call the reviver without `context.source`.
		const parse = JSON.parse
		const spy = vi.spyOn(JSON, 'parse').mockImplementation((text, reviver) =>
			parse(
				text,
				reviver &&
					function (this: unknown, key: string, value: unknown) {
						return reviver.call(this, key, value)
					},
			),
		)
		try {
			expect(coerceArg('f', '{"e": 1e300, "p": 1.5, "s": "9007199254740993"}', 'object')).toEqual({
				e: 1e300,
				p: 1.5,
				s: '9007199254740993',
			})
			expect(() => coerceArg('f', '{"id": 9007199254740993}', 'object')).toThrow(
				'Field "f" has an integer literal outside ±9007199254740991 that would lose precision; send it as a string.',
			)
		} finally {
			spy.mockRestore()
		}
	})
})
