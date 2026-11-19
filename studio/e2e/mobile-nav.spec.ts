// Mobile (390x844) navigation and overflow, run by the `mobile` project only.
import type { Page } from '@playwright/test'

import { expect, test } from './support/fixtures'
import { sel } from './support/selectors'

const overflow = (page: Page) =>
	page.evaluate(() => ({
		scroll: document.documentElement.scrollWidth,
		client: document.documentElement.clientWidth,
	}))

test.describe('mobile nav', () => {
	test('bottom Primary nav is visible and the sidebar is hidden', async ({ page }) => {
		// Scenario: below the md breakpoint only the bottom tab bar shows.
		await page.goto('.')
		await expect(sel.primaryNav(page)).toBeVisible()
		await expect(page.locator('aside')).toBeHidden()
	})

	test('all four links navigate', async ({ page }) => {
		// Scenario: tapping each tab changes the route and heading.
		await page.goto('.')
		const nav = sel.primaryNav(page)
		for (const [name, path, heading] of [
			['Data', '/_studio/data', 'Data Explorer'],
			['REST', '/_studio/rest', 'REST Explorer'],
			['MCP', '/_studio/mcp', 'MCP Explorer'],
			['Overview', '/_studio/', 'Overview'],
		] as const) {
			await nav.getByRole('link', { name }).click()
			await expect(page).toHaveURL(new RegExp(`${path}(\\?.*)?$`))
			await expect(page.getByRole('heading', { name: heading })).toBeVisible()
		}
	})

	test('Overview has no horizontal overflow (D13)', async ({ page }) => {
		// Scenario: document width must not exceed the 390px viewport.
		await page.goto('.')
		await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
		await expect(page.getByText('Version', { exact: true })).toBeVisible()
		expect((await overflow(page)).scroll).toBeLessThanOrEqual(390)
	})

	test('Data Explorer with items selected has no horizontal overflow (D13)', async ({ page }) => {
		// Scenario: wide actions row and metadata values must wrap at 390px.
		await page.goto('/_studio/data?db=prest&schema=public&table=items')
		await expect(page.getByRole('heading', { name: 'Data Explorer' })).toBeVisible()
		await expect(page.getByRole('cell', { name: 'item-1', exact: true }).first()).toBeVisible()
		expect((await overflow(page)).scroll).toBeLessThanOrEqual(390)
	})
})
