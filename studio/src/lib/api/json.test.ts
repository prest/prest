import { describe, expect, it } from 'vitest'
import { formatJsonText, losslessReviver, parseJsonLossless } from '@/lib/api/json'

describe('losslessReviver', () => {
	it('returns source text for numbers that do not round-trip', () => {
		expect(losslessReviver.call({}, 'k', 9007199254740992, { source: '9007199254740993' })).toBe(
			'9007199254740993',
		)
		expect(
			losslessReviver.call({}, 'k', 1.2345678901234567e19, {
				source: '12345678901234567890.1234567891',
			}),
		).toBe('12345678901234567890.1234567891')
		expect(losslessReviver.call({}, 'k', 1.5, { source: '1.50' })).toBe('1.50')
	})

	it('keeps numbers that round-trip', () => {
		expect(losslessReviver.call({}, 'k', 42, { source: '42' })).toBe(42)
		expect(losslessReviver.call({}, 'k', 1.5, { source: '1.5' })).toBe(1.5)
	})

	it('returns the value unchanged without context or source', () => {
		expect(losslessReviver.call({}, 'k', 9007199254740992)).toBe(9007199254740992)
		expect(losslessReviver.call({}, 'k', 7, {})).toBe(7)
	})

	it('leaves non-numbers alone', () => {
		expect(losslessReviver.call({}, 'k', 'a', { source: '"a"' })).toBe('a')
		expect(losslessReviver.call({}, 'k', null, { source: 'null' })).toBe(null)
	})
})

describe('parseJsonLossless number edge cases', () => {
	it('keeps source for literals whose double text differs', () => {
		expect(parseJsonLossless('[1.0,-0,1E3,1e400,0.0000001]')).toEqual([
			'1.0',
			'-0',
			'1E3',
			'1e400',
			'0.0000001',
		])
	})
	it('keeps negative big integers as strings', () => {
		expect(parseJsonLossless('-9007199254740993')).toBe('-9007199254740993')
	})
	it('applies to nested numbers', () => {
		expect(parseJsonLossless('{"a":[9007199254740993,{"b":2,"c":2.50}]}')).toEqual({
			a: ['9007199254740993', { b: 2, c: '2.50' }],
		})
	})
})

describe('parseJsonLossless', () => {
	it('parses documents keeping big literals', () => {
		expect(parseJsonLossless('[{"id":9007199254740993,"n":3,"s":"x"}]')).toEqual([
			{ id: '9007199254740993', n: 3, s: 'x' },
		])
	})
	it('throws on invalid JSON', () => {
		expect(() => parseJsonLossless('{')).toThrow()
	})
})

describe('formatJsonText', () => {
	it('re-indents and preserves big literals', () => {
		const out = formatJsonText('{"a":9007199254740993,"b":[1,2.50,{"c":null}],"d":true}')
		expect(out).toBe(
			'{\n  "a": 9007199254740993,\n  "b": [\n    1,\n    2.50,\n    {\n      "c": null\n    }\n  ],\n  "d": true\n}',
		)
		expect(JSON.parse(out)).toEqual(
			JSON.parse('{"a":9007199254740992,"b":[1,2.5,{"c":null}],"d":true}'),
		)
	})
	it('keeps empty containers compact', () => {
		expect(formatJsonText('{"a":{},"b":[]}')).toBe('{\n  "a": {},\n  "b": []\n}')
		expect(formatJsonText('[]')).toBe('[]')
	})
	it('preserves escaped quotes, brackets in strings and unicode', () => {
		const src = '{"k\\"q":"a\\"b,{[]}:","u":"\\u00e9é"}'
		const out = formatJsonText(src)
		expect(JSON.parse(out)).toEqual(JSON.parse(src))
		expect(out).toContain('"k\\"q": "a\\"b,{[]}:"')
		expect(out).toContain('"\\u00e9é"')
	})
	it('honours a custom indent and strips input whitespace', () => {
		expect(formatJsonText(' { "a" : [ 1 ] } ', 4)).toBe('{\n    "a": [\n        1\n    ]\n}')
	})
	it('returns invalid input unchanged', () => {
		expect(formatJsonText('{oops')).toBe('{oops')
		expect(formatJsonText('')).toBe('')
	})
	it('formats scalars', () => {
		expect(formatJsonText('  9007199254740993 ')).toBe('9007199254740993')
		expect(formatJsonText('"x"')).toBe('"x"')
	})
})
