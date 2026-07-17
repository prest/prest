import { defineConfig, devices } from '@playwright/test'

/**
 * E2E config stub. Real specs live under `e2e/` (added later). By default this
 * assumes a Vite preview server; wire `webServer` to prestd for full-stack runs.
 */
export default defineConfig({
	testDir: './e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	reporter: 'html',
	use: {
		baseURL: process.env.STUDIO_E2E_BASE_URL ?? 'http://localhost:4173/_studio/',
		trace: 'on-first-retry',
	},
	projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
	webServer: process.env.STUDIO_E2E_BASE_URL
		? undefined
		: {
				command: 'pnpm build && pnpm preview --port 4173',
				url: 'http://localhost:4173/_studio/',
				reuseExistingServer: !process.env.CI,
				timeout: 120_000,
			},
})
