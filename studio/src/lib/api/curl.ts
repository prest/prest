/**
 * Build copy-pasteable `curl` commands for a request.
 *
 * The Authorization header is OMITTED by default so users never accidentally
 * copy their bearer token into a shared snippet. It is only included when
 * `includeAuth` is explicitly set and a token is present.
 */

export interface CurlOptions {
	method?: string
	/** Absolute or origin-relative URL to request. */
	url: string
	/** Origin to prefix when `url` is relative (e.g. `http://localhost:3000`). */
	origin?: string
	headers?: Record<string, string>
	body?: string
	/** Opt-in: include `Authorization: Bearer <token>`. */
	includeAuth?: boolean
	token?: string | null
}

/** Single-quote a value for safe inclusion in a POSIX shell command. */
export function shellQuote(value: string): string {
	return `'${value.replace(/'/g, "'\\''")}'`
}

function resolveUrl(url: string, origin?: string): string {
	if (/^https?:\/\//i.test(url)) return url
	if (!origin) return url
	const trimmedOrigin = origin.replace(/\/$/, '')
	const path = url.startsWith('/') ? url : `/${url}`
	return `${trimmedOrigin}${path}`
}

export function buildCurl(options: CurlOptions): string {
	const method = (options.method ?? 'GET').toUpperCase()
	const url = resolveUrl(options.url, options.origin)

	const parts: string[] = ['curl']
	if (method !== 'GET') {
		parts.push('-X', method)
	}
	parts.push(shellQuote(url))

	const headers: Record<string, string> = { Accept: 'application/json', ...options.headers }
	for (const [key, value] of Object.entries(headers)) {
		parts.push('-H', shellQuote(`${key}: ${value}`))
	}

	if (options.includeAuth && options.token) {
		parts.push('-H', shellQuote(`Authorization: Bearer ${options.token}`))
	}

	if (options.body != null && options.body.length > 0) {
		parts.push('-H', shellQuote('Content-Type: application/json'))
		parts.push('--data', shellQuote(options.body))
	}

	return parts.join(' ')
}
