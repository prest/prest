import { describe, expect, it } from 'vitest'
import { buildCurl, shellQuote } from '@/lib/api/curl'

describe('shellQuote', () => {
	it('wraps values in single quotes', () => {
		expect(shellQuote('abc')).toBe("'abc'")
	})

	it('escapes embedded single quotes', () => {
		expect(shellQuote("a'b")).toBe("'a'\\''b'")
	})
})

describe('buildCurl', () => {
	it('builds a GET curl with an Accept header by default', () => {
		const cmd = buildCurl({ url: '/databases' })
		expect(cmd).toBe("curl '/databases' -H 'Accept: application/json'")
	})

	it('prefixes a relative url with the origin', () => {
		const cmd = buildCurl({ url: '/t', origin: 'http://localhost:3000/' })
		expect(cmd).toContain("'http://localhost:3000/t'")
	})

	it('leaves absolute urls untouched', () => {
		const cmd = buildCurl({ url: 'https://api.example.com/t', origin: 'http://x' })
		expect(cmd).toContain("'https://api.example.com/t'")
	})

	it('adds -X for non-GET methods', () => {
		const cmd = buildCurl({ method: 'post', url: '/t', body: '{"a":1}' })
		expect(cmd).toContain('-X POST')
		expect(cmd).toContain("-H 'Content-Type: application/json'")
		expect(cmd).toContain('--data \'{"a":1}\'')
	})

	it('omits the Authorization header unless explicitly requested', () => {
		expect(buildCurl({ url: '/t', token: 'secret' })).not.toContain('Authorization')
		const withAuth = buildCurl({ url: '/t', includeAuth: true, token: 'secret' })
		expect(withAuth).toContain("-H 'Authorization: Bearer secret'")
	})

	it('does not add auth when opted in but no token is present', () => {
		expect(buildCurl({ url: '/t', includeAuth: true, token: null })).not.toContain('Authorization')
	})

	it('merges custom headers over the defaults', () => {
		const cmd = buildCurl({ url: '/t', headers: { Accept: 'text/csv' } })
		expect(cmd).toContain("-H 'Accept: text/csv'")
	})

	it('keeps a relative url when no origin is provided', () => {
		expect(buildCurl({ url: 't' })).toContain("'t'")
	})
})
