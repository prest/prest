/// <reference types="vitest/config" />
import { defineConfig, loadEnv } from 'vite'
import type { IncomingMessage } from 'node:http'
import { fileURLToPath, URL } from 'node:url'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const STUDIO_BASE = '/_studio/'

/**
 * Dev-only proxy. The Studio SPA is served by Vite under `/_studio/`, while all
 * pREST API calls (which the app issues from the origin root, e.g. `/databases`
 * or `/{db}/{schema}/{table}`) must be forwarded to a running prestd instance.
 *
 * A single catch-all proxy is used together with a `bypass` predicate so that
 * arbitrary table data paths are forwarded without having to enumerate them,
 * while Vite keeps ownership of its own dev assets and the SPA shell.
 */
function shouldBeServedByVite(url: string): boolean {
	// The Studio API namespace must always hit the backend.
	if (url.startsWith('/_studio/api')) return false
	// Everything else under the Studio base is the SPA / its dev assets.
	if (url === '/_studio' || url.startsWith('/_studio/')) return true
	// Vite internals and source modules.
	if (
		url.startsWith('/@') ||
		url.startsWith('/src/') ||
		url.startsWith('/node_modules/') ||
		url.startsWith('/.vite') ||
		url === '/favicon.ico'
	) {
		return true
	}
	// Root redirect target handled by Vite.
	if (url === '/') return true
	return false
}

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '')
	const target = env.VITE_PREST_PROXY_TARGET || 'http://localhost:3000'

	return {
		base: STUDIO_BASE,
		plugins: [react(), tailwindcss()],
		resolve: {
			alias: {
				'@': fileURLToPath(new URL('./src', import.meta.url)),
			},
		},
		server: {
			port: 5173,
			proxy: {
				'/': {
					target,
					changeOrigin: true,
					bypass: (req: IncomingMessage) => {
						const url = req.url ?? ''
						return shouldBeServedByVite(url) ? url : undefined
					},
				},
			},
		},
		build: {
			outDir: fileURLToPath(new URL('../internal/studio/dist', import.meta.url)),
			emptyOutDir: true,
			sourcemap: false,
		},
		test: {
			globals: true,
			environment: 'jsdom',
			setupFiles: ['./src/test/setup.ts'],
			include: ['src/**/*.{test,spec}.{ts,tsx}'],
			exclude: ['e2e/**', 'node_modules/**'],
			css: true,
			coverage: {
				provider: 'v8',
				reporter: ['text', 'html', 'lcov'],
				// Coverage gate is focused on the critical, framework-agnostic logic in
				// `lib/` (API/MCP clients, auth, errors, validation). UI components are
				// still tested for correctness but excluded from the hard threshold, as
				// exhaustive rendering coverage is out of scope for this MVP.
				include: ['src/lib/**/*.{ts,tsx}'],
				exclude: ['src/**/*.test.{ts,tsx}', 'src/test/**', 'src/**/*.d.ts', 'src/vite-env.d.ts'],
				thresholds: {
					statements: 90,
					lines: 90,
					functions: 90,
					branches: 84,
				},
			},
		},
	}
})
