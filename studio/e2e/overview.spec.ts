// Overview page: server probes, catalog counts (D8), metadata, quick links and the exposure mismatch.
import type { Locator, Page } from '@playwright/test'

import { expect, test } from './support/fixtures'

/** The count card whose caption is exactly `label`. */
function countCard(page: Page, label: string): Locator {
	return page
		.locator('div.text-2xl')
		.filter({ has: page.locator('xpath=following-sibling::div[text()="' + label + '"]') })
}

test.describe('overview', () => {
	test('health, ready and MCP probes report OK', async ({ page }) => {
		// Scenario: all three probe rows resolve to HTTP 200 against the main instance.
		await page.goto('.')
		await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
		for (const label of ['Health (/_health)', 'Ready (/_ready)', 'MCP (/_mcp)']) {
			await expect(page.getByText(label).locator('xpath=..')).toContainText('HTTP 200')
		}
	})

	test('catalog counts exclude system schemas and sequences (D8)', async ({ page }) => {
		// Scenario: fixture has schemas public+shop and six browsable relations; sequences and pg_catalog are hidden.
		await page.goto('.')
		await expect(page.getByText('Databases', { exact: true })).toBeVisible()
		await expect(countCard(page, 'Databases')).toHaveText('1')
		await expect(countCard(page, 'Schemas')).toHaveText('2')
		await expect(countCard(page, 'Tables')).toHaveText('6')
	})

	test('server metadata shows version, commit, API base and MCP endpoint', async ({ page }) => {
		// Scenario: values come from /_studio/api/meta; the harness binary reports base `/` and `/_mcp`.
		await page.goto('.')
		const row = (label: string) => page.getByText(label, { exact: true }).locator('xpath=..')
		await expect(row('Version')).not.toContainText('—')
		await expect(row('Commit')).not.toContainText('—')
		await expect(row('API base')).toContainText('/')
		await expect(row('MCP endpoint')).toContainText('/_mcp')
	})

	test('bearer token status is Not set by default', async ({ page }) => {
		// Scenario: fresh context, no token stored.
		await page.goto('.')
		await expect(page.getByText('Bearer token').locator('xpath=..')).toContainText('Not set')
	})

	for (const [label, path] of [
		['Data Explorer', '/_studio/data'],
		['REST Explorer', '/_studio/rest'],
		['MCP Explorer', '/_studio/mcp'],
	] as const) {
		test(`quick link ${label} navigates to ${path}`, async ({ page }) => {
			// Scenario: clicking a quick-link card routes inside the SPA.
			await page.goto('.')
			await page
				.getByRole('link', { name: new RegExp(label) })
				.first()
				.click()
			await expect(page).toHaveURL(new RegExp(`${path}(\\?.*)?$`))
			await expect(page.getByRole('heading', { name: label })).toBeVisible()
		})
	}

	test('restricted /tables listing shows a dash on Overview and a notice in Data Explorer', async ({
		page,
	}) => {
		// Scenario: /tables answers 401 {"error":"unauthorized listing"} (exposure mismatch), not a real auth failure.
		await page.route('**/tables', (route) =>
			route.fulfill({
				status: 401,
				contentType: 'application/json',
				body: '{"error":"unauthorized listing"}',
			}),
		)
		await page.goto('.')
		await expect(countCard(page, 'Tables (restricted)')).toHaveText('—')
		await page.getByRole('link', { name: 'Data', exact: true }).first().click()
		await expect(page.getByText('Table listing is restricted.')).toBeVisible()
	})
})
