import { describe, expect, it } from 'vitest'
import { ApiError, kindFromStatus, messageFromStatus, toErrorMessage } from '@/lib/errors'

describe('kindFromStatus', () => {
	it.each([
		[401, 'unauthorized'],
		[403, 'forbidden'],
		[404, 'not_found'],
		[400, 'bad_request'],
		[422, 'bad_request'],
		[418, 'bad_request'],
		[500, 'server'],
		[503, 'server'],
		[200, 'unknown'],
	])('maps %i to %s', (status, kind) => {
		expect(kindFromStatus(status)).toBe(kind)
	})
})

describe('messageFromStatus', () => {
	it('returns specific messages for known statuses', () => {
		expect(messageFromStatus(401)).toMatch(/Unauthorized/)
		expect(messageFromStatus(403)).toMatch(/Forbidden/)
		expect(messageFromStatus(404)).toMatch(/Not found/)
		expect(messageFromStatus(400)).toMatch(/Bad request/)
		expect(messageFromStatus(422)).toMatch(/Bad request/)
		expect(messageFromStatus(500)).toMatch(/Server error/)
	})

	it('uses the fallback for other statuses', () => {
		expect(messageFromStatus(301, 'moved')).toBe('moved')
		expect(messageFromStatus(301)).toMatch(/Request failed/)
	})
})

describe('ApiError', () => {
	it('captures kind, status, body, url and cause', () => {
		const cause = new Error('boom')
		const err = new ApiError('nope', {
			kind: 'server',
			status: 500,
			body: { error: 'x' },
			url: '/t',
			cause,
		})
		expect(err.name).toBe('ApiError')
		expect(err.kind).toBe('server')
		expect(err.status).toBe(500)
		expect(err.url).toBe('/t')
		expect((err as { cause?: unknown }).cause).toBe(cause)
	})

	it('defaults to unknown kind', () => {
		expect(new ApiError('x').kind).toBe('unknown')
	})

	it('exposes isAuth for 401/403 kinds', () => {
		expect(new ApiError('x', { kind: 'unauthorized' }).isAuth).toBe(true)
		expect(new ApiError('x', { kind: 'forbidden' }).isAuth).toBe(true)
		expect(new ApiError('x', { kind: 'server' }).isAuth).toBe(false)
	})

	it('exposes isDisabled for 403/404 kinds', () => {
		expect(new ApiError('x', { kind: 'forbidden' }).isDisabled).toBe(true)
		expect(new ApiError('x', { kind: 'not_found' }).isDisabled).toBe(true)
		expect(new ApiError('x', { kind: 'network' }).isDisabled).toBe(false)
	})
})

describe('toErrorMessage', () => {
	it('handles ApiError, Error, string and unknown', () => {
		expect(toErrorMessage(new ApiError('api'))).toBe('api')
		expect(toErrorMessage(new Error('plain'))).toBe('plain')
		expect(toErrorMessage('str')).toBe('str')
		expect(toErrorMessage(42)).toBe('Unexpected error.')
	})
})
