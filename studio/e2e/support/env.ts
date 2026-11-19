// Shared e2e environment: ports, base URLs and the auth instance JWT key.
const num = (name: string, fallback: number): number => Number(process.env[name] ?? fallback)

export const PG_PORT = num('STUDIO_E2E_PG_PORT', 55432)
export const MAIN_PORT = num('STUDIO_E2E_PORT', 39991)
export const AUTH_PORT = num('STUDIO_E2E_AUTH_PORT', 39992)

export const MAIN_ORIGIN = process.env.STUDIO_E2E_BASE_URL
	? new URL(process.env.STUDIO_E2E_BASE_URL).origin
	: `http://localhost:${MAIN_PORT}`
export const AUTH_ORIGIN = process.env.STUDIO_E2E_AUTH_BASE_URL
	? new URL(process.env.STUDIO_E2E_AUTH_BASE_URL).origin
	: `http://localhost:${AUTH_PORT}`

export const MAIN_BASE_URL = `${MAIN_ORIGIN}/_studio/`
export const AUTH_BASE_URL = `${AUTH_ORIGIN}/_studio/`

/** Must match `jwt.key` in fixtures/prest-auth.toml (HS256 needs 32+ chars). */
export const E2E_JWT_KEY = 'studio-e2e-jwt-key-0123456789-abcdef'

export const STACK_MODE = process.env.STUDIO_E2E_STACK === '1'
