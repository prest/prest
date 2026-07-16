/**
 * Typed error hierarchy shared by the API and MCP clients so UI code can
 * discriminate failures (auth, permission, not-found, timeout, network, server)
 * without inspecting raw fetch internals.
 */

export type ApiErrorKind =
	| 'network'
	| 'timeout'
	| 'unauthorized'
	| 'forbidden'
	| 'not_found'
	| 'bad_request'
	| 'server'
	| 'parse'
	| 'unknown'

export class ApiError extends Error {
	readonly kind: ApiErrorKind
	readonly status?: number
	readonly body?: unknown
	readonly url?: string

	constructor(
		message: string,
		options: {
			kind: ApiErrorKind
			status?: number
			body?: unknown
			url?: string
			cause?: unknown
		} = {
			kind: 'unknown',
		},
	) {
		super(message)
		this.name = 'ApiError'
		this.kind = options.kind
		this.status = options.status
		this.body = options.body
		this.url = options.url
		if (options.cause !== undefined) {
			;(this as { cause?: unknown }).cause = options.cause
		}
	}

	/** True for 401/403 – the caller likely needs to set or fix the bearer token. */
	get isAuth(): boolean {
		return this.kind === 'unauthorized' || this.kind === 'forbidden'
	}

	/** 403/404 responses are treated as "feature disabled" during discovery. */
	get isDisabled(): boolean {
		return this.kind === 'forbidden' || this.kind === 'not_found'
	}
}

/** Map an HTTP status code to a coarse {@link ApiErrorKind}. */
export function kindFromStatus(status: number): ApiErrorKind {
	if (status === 401) return 'unauthorized'
	if (status === 403) return 'forbidden'
	if (status === 404) return 'not_found'
	if (status === 400 || status === 422) return 'bad_request'
	if (status >= 500) return 'server'
	if (status >= 400) return 'bad_request'
	return 'unknown'
}

/** Human-friendly, non-leaking message for a status code. */
export function messageFromStatus(status: number, fallback?: string): string {
	switch (status) {
		case 401:
			return 'Unauthorized – set or update your bearer token.'
		case 403:
			return 'Forbidden – your token lacks access to this resource.'
		case 404:
			return 'Not found – the resource or endpoint is unavailable.'
		case 400:
		case 422:
			return 'Bad request – check filters and parameters.'
		default:
			if (status >= 500) return `Server error (${status}).`
			return fallback ?? `Request failed (${status}).`
	}
}

/** Extract a safe, displayable message from any thrown value. */
export function toErrorMessage(err: unknown): string {
	if (err instanceof ApiError) return err.message
	if (err instanceof Error) return err.message
	if (typeof err === 'string') return err
	return 'Unexpected error.'
}
