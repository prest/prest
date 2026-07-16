import { beforeEach, describe, expect, it, vi } from 'vitest'
import { TokenStore, maskToken } from '@/lib/auth/token'

const STORAGE_KEY = 'prest.studio.token'

describe('TokenStore', () => {
	beforeEach(() => {
		vi.unstubAllGlobals()
		sessionStorage.clear()
		vi.restoreAllMocks()
	})

	it('starts empty when nothing is persisted', () => {
		const store = new TokenStore()
		expect(store.getToken()).toBeNull()
		expect(store.hasToken()).toBe(false)
		expect(store.isRemembered()).toBe(false)
		expect(store.getSnapshot()).toEqual({ token: null, remembered: false })
	})

	it('loads a persisted token on construction', () => {
		sessionStorage.setItem(STORAGE_KEY, 'persisted')
		const store = new TokenStore()
		expect(store.getToken()).toBe('persisted')
		expect(store.isRemembered()).toBe(true)
	})

	it('sets a memory-only token by default and does not persist it', () => {
		const store = new TokenStore()
		store.set('mem')
		expect(store.getToken()).toBe('mem')
		expect(store.isRemembered()).toBe(false)
		expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
	})

	it('persists to sessionStorage when remember is true', () => {
		const store = new TokenStore()
		store.set('remembered', true)
		expect(sessionStorage.getItem(STORAGE_KEY)).toBe('remembered')
		expect(store.isRemembered()).toBe(true)
	})

	it('removes a previously persisted token when re-set without remember', () => {
		const store = new TokenStore()
		store.set('a', true)
		store.set('b', false)
		expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
		expect(store.getToken()).toBe('b')
	})

	it('trims whitespace and clears on an empty value', () => {
		const store = new TokenStore()
		store.set('  spaced  ', true)
		expect(store.getToken()).toBe('spaced')
		store.set('   ')
		expect(store.getToken()).toBeNull()
		expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
	})

	it('clears the token and persistence', () => {
		const store = new TokenStore()
		store.set('x', true)
		store.clear()
		expect(store.getToken()).toBeNull()
		expect(store.isRemembered()).toBe(false)
		expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
	})

	it('notifies subscribers on change and stops after unsubscribe', () => {
		const store = new TokenStore()
		const listener = vi.fn()
		const unsubscribe = store.subscribe(listener)
		store.set('x')
		expect(listener).toHaveBeenCalledTimes(1)
		expect(store.getSnapshot().token).toBe('x')
		unsubscribe()
		store.set('y')
		expect(listener).toHaveBeenCalledTimes(1)
	})

	it('falls back to memory-only when storage is unavailable', () => {
		vi.stubGlobal('sessionStorage', {
			getItem: () => null,
			setItem: () => {
				throw new Error('denied')
			},
			removeItem: () => undefined,
		})
		const store = new TokenStore()
		store.set('x', true)
		expect(store.getToken()).toBe('x')
		expect(store.isRemembered()).toBe(false)
	})
})

describe('maskToken', () => {
	it('returns empty string for empty input', () => {
		expect(maskToken(null)).toBe('')
		expect(maskToken(undefined)).toBe('')
		expect(maskToken('')).toBe('')
	})

	it('fully masks short tokens', () => {
		expect(maskToken('abcd')).toBe('••••')
	})

	it('keeps a prefix and suffix for long tokens', () => {
		const masked = maskToken('abcdefghijkl')
		expect(masked.startsWith('abcd')).toBe(true)
		expect(masked.endsWith('ijkl')).toBe(true)
		expect(masked).toContain('•')
	})
})
