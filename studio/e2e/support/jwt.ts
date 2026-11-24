// Minimal HS256 JWT signer (node:crypto) for the auth instance.
import { createHmac } from 'node:crypto'

import { E2E_JWT_KEY } from './env'

const b64url = (input: string | Buffer): string => Buffer.from(input).toString('base64url')

export function signToken(claims: Record<string, unknown> = {}, key: string = E2E_JWT_KEY): string {
	const header = b64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
	const exp = Math.floor(Date.now() / 1000) + 3600
	const payload = b64url(JSON.stringify({ username: 'e2e', exp, ...claims }))
	const sig = createHmac('sha256', key).update(`${header}.${payload}`).digest('base64url')
	return `${header}.${payload}.${sig}`
}
