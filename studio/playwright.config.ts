import { existsSync } from 'node:fs'
import { resolve } from 'node:path'

import { defineConfig, devices } from '@playwright/test'

import { AUTH_PORT, MAIN_PORT, PG_PORT } from './e2e/support/env'

/**
 * Three modes:
 * - stack (`STUDIO_E2E_STACK=1`): two prestd instances (main + auth) from `STUDIO_E2E_BIN`
 *   against the compose Postgres (`make studio-e2e-up`); runs every spec.
 * - external (`STUDIO_E2E_BASE_URL`): an already running server; no webServer.
 * - preview (default): `smoke.spec.ts` against `vite preview` of `.e2e-preview`, never `internal/studio/dist`.
 */
const stack = process.env.STUDIO_E2E_STACK === '1'
const external = !!process.env.STUDIO_E2E_BASE_URL
const bin = resolve(process.env.STUDIO_E2E_BIN ?? 'e2e/.bin/prestd')
const conf = (name: string) => resolve('e2e/fixtures', name)

if (stack && !external && !existsSync(bin)) {
	throw new Error(`prestd binary not found at ${bin}; run make studio-e2e-up first`)
}

const instance = (port: number, confFile: string) => ({
	command: bin,
	env: {
		PREST_CONF: conf(confFile),
		PREST_HTTP_PORT: String(port),
		PREST_PG_PORT: String(PG_PORT),
	},
	url: `http://localhost:${port}/_ready`,
	reuseExistingServer: false,
	stdout: 'pipe' as const,
	gracefulShutdown: { signal: 'SIGTERM' as const, timeout: 5000 },
	timeout: 60_000,
})

const desktop = {
	name: 'desktop',
	use: {
		...devices['Desktop Chrome'],
		permissions: ['clipboard-read', 'clipboard-write'],
	},
	testIgnore: /mobile-nav\.spec\.ts/,
}
const mobile = {
	name: 'mobile',
	use: {
		...devices['Desktop Chrome'],
		viewport: { width: 390, height: 844 },
		isMobile: true,
		hasTouch: true,
	},
	testMatch: /mobile-nav\.spec\.ts/,
}

const fullStack = stack || external

export default defineConfig({
	testDir: './e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 1 : 2,
	globalTimeout: 10 * 60_000,
	reporter: process.env.CI
		? [['html', { open: 'never' }], ['list']]
		: [['list'], ['html', { open: 'never' }]],
	use: {
		baseURL: external
			? process.env.STUDIO_E2E_BASE_URL
			: stack
				? `http://localhost:${MAIN_PORT}/_studio/`
				: 'http://localhost:4173/_studio/',
		trace: 'on-first-retry',
	},
	projects: fullStack
		? [desktop, mobile]
		: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] }, testMatch: /smoke\.spec\.ts/ }],
	webServer: external
		? undefined
		: stack
			? [instance(MAIN_PORT, 'prest-main.toml'), instance(AUTH_PORT, 'prest-auth.toml')]
			: {
					command:
						'tsc --noEmit && vite build --outDir .e2e-preview --emptyOutDir && vite preview --outDir .e2e-preview --port 4173 --strictPort',
					url: 'http://localhost:4173/_studio/',
					reuseExistingServer: !process.env.CI,
					timeout: 180_000,
				},
})
