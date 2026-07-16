import { expect, test } from '@playwright/test'

/**
 * Minimal smoke test: the Studio SPA shell must render at `/_studio/`.
 *
 * A full-stack run needs a reachable prestd (the default `webServer` in
 * `playwright.config.ts` builds and previews the SPA). When only the static
 * shell is available, the deeper API-backed assertions are skipped.
 */
test('studio shell loads at /_studio/', async ({ page }) => {
	const response = await page.goto('/')
	expect(response?.ok()).toBeTruthy()

	await expect(page.locator('#root')).toBeVisible()
	await expect(page.getByText('pREST', { exact: false }).first()).toBeVisible()
})

test.describe('backend-dependent', () => {
	test.skip(
		!process.env.PREST_TEST_URL,
		'Set PREST_TEST_URL to run against a live prestd instance.',
	)

	test('overview shows navigation', async ({ page }) => {
		await page.goto('/')
		await expect(page.getByRole('navigation', { name: 'Primary' }).first()).toBeVisible()
	})
})
