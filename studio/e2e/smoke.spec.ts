import { expect, test } from '@playwright/test'

/** Minimal smoke test: the Studio SPA shell must render at `/_studio/`. Stack specs live in their own files. */
test('studio shell loads at /_studio/', async ({ page }) => {
	const response = await page.goto('/_studio/')
	expect(response?.ok()).toBeTruthy()

	await expect(page.locator('#root')).toBeVisible()
	await expect(page.getByText('pREST', { exact: false }).first()).toBeVisible()
})
