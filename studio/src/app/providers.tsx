import * as React from 'react'
import { QueryClient, QueryClientProvider, useQueryClient } from '@tanstack/react-query'
import { PrestClient } from '@/lib/api/client'
import { tokenStore, type TokenStore, type TokenSnapshot } from '@/lib/auth/token'

/* -------------------------------------------------------------------------- */
/* Theme                                                                      */
/* -------------------------------------------------------------------------- */

export type Theme = 'light' | 'dark' | 'system'
export type ResolvedTheme = 'light' | 'dark'

const THEME_KEY = 'prest.studio.theme'

interface ThemeContextValue {
	theme: Theme
	resolvedTheme: ResolvedTheme
	setTheme: (theme: Theme) => void
}

const ThemeContext = React.createContext<ThemeContextValue | null>(null)

function readStoredTheme(): Theme {
	try {
		const v = globalThis.localStorage?.getItem(THEME_KEY)
		if (v === 'light' || v === 'dark' || v === 'system') return v
	} catch {
		/* ignore storage errors */
	}
	return 'system'
}

function systemPrefersDark(): boolean {
	return globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
	const [theme, setThemeState] = React.useState<Theme>(readStoredTheme)
	const [systemDark, setSystemDark] = React.useState<boolean>(systemPrefersDark)

	React.useEffect(() => {
		const mql = globalThis.matchMedia?.('(prefers-color-scheme: dark)')
		if (!mql) return
		const onChange = () => setSystemDark(mql.matches)
		mql.addEventListener('change', onChange)
		return () => mql.removeEventListener('change', onChange)
	}, [])

	const resolvedTheme: ResolvedTheme = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme

	React.useEffect(() => {
		const root = document.documentElement
		root.classList.toggle('dark', resolvedTheme === 'dark')
		root.style.colorScheme = resolvedTheme
	}, [resolvedTheme])

	const setTheme = React.useCallback((next: Theme) => {
		setThemeState(next)
		try {
			globalThis.localStorage?.setItem(THEME_KEY, next)
		} catch {
			/* ignore storage errors */
		}
	}, [])

	const value = React.useMemo<ThemeContextValue>(
		() => ({ theme, resolvedTheme, setTheme }),
		[theme, resolvedTheme, setTheme],
	)

	return <ThemeContext value={value}>{children}</ThemeContext>
}

export function useTheme(): ThemeContextValue {
	const ctx = React.useContext(ThemeContext)
	if (!ctx) throw new Error('useTheme must be used within <ThemeProvider>')
	return ctx
}

/* -------------------------------------------------------------------------- */
/* Auth                                                                       */
/* -------------------------------------------------------------------------- */

interface AuthContextValue extends TokenSnapshot {
	hasToken: boolean
	setToken: (token: string, remember?: boolean) => void
	clearToken: () => void
}

const AuthContext = React.createContext<AuthContextValue | null>(null)

export function AuthProvider({
	store = tokenStore,
	children,
}: {
	store?: TokenStore
	children: React.ReactNode
}) {
	const queryClient = useQueryClient()
	const snapshot = React.useSyncExternalStore(store.subscribe, store.getSnapshot, store.getSnapshot)

	const setToken = React.useCallback(
		(token: string, remember?: boolean) => {
			store.set(token, remember)
			queryClient.clear()
		},
		[store, queryClient],
	)

	const clearToken = React.useCallback(() => {
		store.clear()
		queryClient.clear()
	}, [store, queryClient])

	const value = React.useMemo<AuthContextValue>(
		() => ({
			...snapshot,
			hasToken: snapshot.token !== null && snapshot.token.length > 0,
			setToken,
			clearToken,
		}),
		[snapshot, setToken, clearToken],
	)

	return <AuthContext value={value}>{children}</AuthContext>
}

export function useAuth(): AuthContextValue {
	const ctx = React.useContext(AuthContext)
	if (!ctx) throw new Error('useAuth must be used within <AuthProvider>')
	return ctx
}

/* -------------------------------------------------------------------------- */
/* pREST client                                                               */
/* -------------------------------------------------------------------------- */

const PrestClientContext = React.createContext<PrestClient | null>(null)

export function PrestClientProvider({
	client,
	children,
}: {
	client?: PrestClient
	children: React.ReactNode
}) {
	// A single client instance reads the live token on every request, so it does
	// not need to be recreated when the token changes.
	const instance = React.useMemo(
		() => client ?? new PrestClient({ getToken: () => tokenStore.getToken() }),
		[client],
	)
	return <PrestClientContext value={instance}>{children}</PrestClientContext>
}

export function usePrestClient(): PrestClient {
	const ctx = React.useContext(PrestClientContext)
	if (!ctx) throw new Error('usePrestClient must be used within <PrestClientProvider>')
	return ctx
}

/* -------------------------------------------------------------------------- */
/* Query client                                                               */
/* -------------------------------------------------------------------------- */

export function createQueryClient(): QueryClient {
	return new QueryClient({
		defaultOptions: {
			queries: {
				retry: false,
				refetchOnWindowFocus: false,
				staleTime: 30_000,
			},
		},
	})
}

/* -------------------------------------------------------------------------- */
/* Root composition                                                           */
/* -------------------------------------------------------------------------- */

export function AppProviders({
	children,
	queryClient,
}: {
	children: React.ReactNode
	queryClient?: QueryClient
}) {
	const [client] = React.useState(() => queryClient ?? createQueryClient())
	return (
		<QueryClientProvider client={client}>
			<ThemeProvider>
				<AuthProvider>
					<PrestClientProvider>{children}</PrestClientProvider>
				</AuthProvider>
			</ThemeProvider>
		</QueryClientProvider>
	)
}
