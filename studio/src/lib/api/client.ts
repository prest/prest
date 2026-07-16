/**
 * Minimal, typed fetch client for pREST.
 *
 * Responsibilities:
 *  - Prefix requests with a configurable API base.
 *  - Inject `Authorization: Bearer <token>` when a token provider yields one.
 *  - Enforce a request timeout via AbortController.
 *  - Normalize failures into {@link ApiError} with a discriminable `kind`.
 */

import { ApiError, kindFromStatus, messageFromStatus } from '@/lib/errors'

export type TokenProvider = () => string | null

export interface PrestClientOptions {
	/** Base path prefixed to relative request paths. Defaults to ''. */
	baseUrl?: string
	/** Returns the current bearer token, or null. */
	getToken?: TokenProvider
	/** Injected fetch (tests). Defaults to global fetch. */
	fetchImpl?: typeof fetch
	/** Request timeout in ms. Defaults to 30000. */
	timeoutMs?: number
}

export interface RequestOptions {
	method?: string
	query?: string | URLSearchParams
	headers?: Record<string, string>
	body?: unknown
	signal?: AbortSignal
	/** Override default timeout for this request. */
	timeoutMs?: number
	/** Skip attaching the Authorization header. */
	noAuth?: boolean
}

export interface ProbeResult {
	status: number
	ok: boolean
}

export interface JsonResponse<T> {
	data: T
	status: number
	headers: Headers
	durationMs: number
}

const DEFAULT_TIMEOUT = 30_000

export class PrestClient {
	private readonly baseUrl: string
	private readonly getToken: TokenProvider
	private readonly fetchImpl: typeof fetch
	private readonly timeoutMs: number

	constructor(options: PrestClientOptions = {}) {
		this.baseUrl = (options.baseUrl ?? '').replace(/\/$/, '')
		this.getToken = options.getToken ?? (() => null)
		this.fetchImpl = options.fetchImpl ?? globalThis.fetch.bind(globalThis)
		this.timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT
	}

	/** Resolve a relative path (and optional query) into a request URL. */
	resolve(path: string, query?: string | URLSearchParams): string {
		const p = path.startsWith('/') || /^https?:\/\//i.test(path) ? path : `/${path}`
		const base = /^https?:\/\//i.test(p) ? p : `${this.baseUrl}${p}`
		if (!query) return base
		const qs = typeof query === 'string' ? query : query.toString()
		if (!qs) return base
		return base.includes('?') ? `${base}&${qs}` : `${base}?${qs}`
	}

	private buildHeaders(opts: RequestOptions): Headers {
		const headers = new Headers({ Accept: 'application/json' })
		if (opts.body != null) headers.set('Content-Type', 'application/json')
		for (const [k, v] of Object.entries(opts.headers ?? {})) headers.set(k, v)
		if (!opts.noAuth) {
			const token = this.getToken()
			if (token) headers.set('Authorization', `Bearer ${token}`)
		}
		return headers
	}

	private async run(path: string, opts: RequestOptions): Promise<Response> {
		const url = this.resolve(path, opts.query)
		const controller = new AbortController()
		const timeout = opts.timeoutMs ?? this.timeoutMs
		const timer = setTimeout(() => controller.abort(), timeout)

		// Propagate external cancellation into our controller.
		if (opts.signal) {
			if (opts.signal.aborted) controller.abort()
			else opts.signal.addEventListener('abort', () => controller.abort(), { once: true })
		}

		try {
			return await this.fetchImpl(url, {
				method: opts.method ?? 'GET',
				headers: this.buildHeaders(opts),
				body: opts.body != null ? JSON.stringify(opts.body) : undefined,
				signal: controller.signal,
			})
		} catch (err) {
			if (controller.signal.aborted) {
				throw new ApiError(`Request timed out after ${timeout}ms.`, {
					kind: 'timeout',
					url,
					cause: err,
				})
			}
			throw new ApiError('Network error – could not reach the server.', {
				kind: 'network',
				url,
				cause: err,
			})
		} finally {
			clearTimeout(timer)
		}
	}

	/** Fire a request and return only status info (empty-body endpoints). */
	async probe(path: string, opts: RequestOptions = {}): Promise<ProbeResult> {
		const res = await this.run(path, opts)
		return { status: res.status, ok: res.ok }
	}

	/** Request JSON and throw {@link ApiError} on non-2xx or parse failure. */
	async requestJson<T = unknown>(
		path: string,
		opts: RequestOptions = {},
	): Promise<JsonResponse<T>> {
		const start = now()
		const res = await this.run(path, opts)
		const durationMs = Math.round(now() - start)
		const url = this.resolve(path, opts.query)

		if (!res.ok) {
			const body = await safeReadBody(res)
			throw new ApiError(messageFromStatus(res.status, bodyMessage(body)), {
				kind: kindFromStatus(res.status),
				status: res.status,
				body,
				url,
			})
		}

		const text = await res.text()
		const data = parseJson<T>(text, url)
		return { data, status: res.status, headers: res.headers, durationMs }
	}

	/** Like {@link requestJson} but returns raw text alongside metadata. */
	async requestRaw(
		path: string,
		opts: RequestOptions = {},
	): Promise<{ text: string; status: number; ok: boolean; headers: Headers; durationMs: number }> {
		const start = now()
		const res = await this.run(path, opts)
		const durationMs = Math.round(now() - start)
		const text = await res.text()
		return { text, status: res.status, ok: res.ok, headers: res.headers, durationMs }
	}
}

function now(): number {
	return typeof performance !== 'undefined' ? performance.now() : Date.now()
}

async function safeReadBody(res: Response): Promise<unknown> {
	try {
		const text = await res.text()
		if (!text) return undefined
		try {
			return JSON.parse(text)
		} catch {
			return text
		}
	} catch {
		return undefined
	}
}

function bodyMessage(body: unknown): string | undefined {
	if (body && typeof body === 'object' && 'error' in body) {
		const e = (body as { error: unknown }).error
		if (typeof e === 'string') return e
	}
	if (typeof body === 'string' && body.length > 0 && body.length < 300) return body
	return undefined
}

function parseJson<T>(text: string, url: string): T {
	if (!text) return undefined as T
	try {
		return JSON.parse(text) as T
	} catch (err) {
		throw new ApiError('Failed to parse JSON response.', { kind: 'parse', url, cause: err })
	}
}
