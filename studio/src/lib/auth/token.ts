/**
 * Bearer-token store for Studio.
 *
 * Security posture:
 *  - Default persistence is in-memory only (cleared on reload).
 *  - Opt-in "remember this tab" uses `sessionStorage` (scoped to the tab and
 *    cleared when it closes). `localStorage` is NEVER used.
 *  - The token is treated as sensitive: it is masked for display and only
 *    added to outgoing requests / curl commands when explicitly present.
 */

const STORAGE_KEY = 'prest.studio.token'

export interface TokenSnapshot {
	/** The raw bearer token, or null when unset. */
	token: string | null
	/** Whether the token is persisted to sessionStorage for this tab. */
	remembered: boolean
}

type Listener = () => void

function safeSessionStorage(): Storage | null {
	try {
		if (typeof globalThis === 'undefined') return null
		const s = (globalThis as { sessionStorage?: Storage }).sessionStorage
		if (!s) return null
		// Touch to surface access errors (e.g. disabled storage) eagerly.
		const probe = '__prest_probe__'
		s.setItem(probe, '1')
		s.removeItem(probe)
		return s
	} catch {
		return null
	}
}

export class TokenStore {
	private token: string | null = null
	private remembered = false
	private listeners = new Set<Listener>()
	private snapshot: TokenSnapshot = { token: null, remembered: false }

	constructor() {
		const store = safeSessionStorage()
		if (store) {
			const existing = store.getItem(STORAGE_KEY)
			if (existing) {
				this.token = existing
				this.remembered = true
			}
		}
		this.recompute()
	}

	private recompute(): void {
		this.snapshot = { token: this.token, remembered: this.remembered }
	}

	private emit(): void {
		this.recompute()
		for (const l of this.listeners) l()
	}

	getSnapshot = (): TokenSnapshot => this.snapshot

	subscribe = (listener: Listener): (() => void) => {
		this.listeners.add(listener)
		return () => {
			this.listeners.delete(listener)
		}
	}

	getToken(): string | null {
		return this.token
	}

	hasToken(): boolean {
		return this.token !== null && this.token.length > 0
	}

	isRemembered(): boolean {
		return this.remembered
	}

	/**
	 * Set the active token. When `remember` is true the token is persisted to
	 * sessionStorage for the current tab; otherwise it lives in memory only and
	 * any previously persisted value is removed.
	 */
	set(token: string, remember = false): void {
		const trimmed = token.trim()
		if (!trimmed) {
			this.clear()
			return
		}
		this.token = trimmed
		this.remembered = remember
		const store = safeSessionStorage()
		if (store) {
			try {
				if (remember) {
					store.setItem(STORAGE_KEY, trimmed)
				} else {
					store.removeItem(STORAGE_KEY)
				}
			} catch {
				// Probe succeeded but a later write failed — keep memory token only.
				this.remembered = false
			}
		} else {
			this.remembered = false
		}
		this.emit()
	}

	clear(): void {
		this.token = null
		this.remembered = false
		const store = safeSessionStorage()
		if (store) {
			try {
				store.removeItem(STORAGE_KEY)
			} catch {
				/* still clear in-memory state and notify; persistence may be stale */
			}
		}
		this.emit()
	}
}

/**
 * Mask a token for display, keeping a short prefix/suffix for recognizability.
 * Returns an empty string for empty input.
 */
export function maskToken(token: string | null | undefined): string {
	if (!token) return ''
	if (token.length <= 8) return '•'.repeat(token.length)
	const head = token.slice(0, 4)
	const tail = token.slice(-4)
	return `${head}${'•'.repeat(Math.max(4, token.length - 8))}${tail}`
}

/** Shared singleton store used by the app. */
export const tokenStore = new TokenStore()
