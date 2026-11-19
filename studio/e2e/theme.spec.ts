// Theme toggle: cycle order, `html.dark` class, localStorage persistence and system colour scheme.
import type { Page } from '@playwright/test'

import { expect, test } from './support/fixtures'
import { sel } from './support/selectors'

const KEY = 'prest.studio.theme'
const isDark = (page: Page) =>
	page.evaluate(() => document.documentElement.classList.contains('dark'))
const stored = (page: Page) => page.evaluate((k) => localStorage.getItem(k), KEY)

test.describe('theme', () => {
	test.use({ colorScheme: 'light' })

	test('toggle cycles system -> light -> dark -> system and updates the html class', async ({
		page,
	}) => {
		// Scenario: three clicks walk the whole cycle; only `dark` carries the class (OS scheme is light).
		await page.goto('.')
		const toggle = sel.themeToggle(page)
		await expect(toggle).toHaveAttribute('aria-label', /^Theme: system\./)
		expect(await isDark(page)).toBe(false)

		await toggle.click()
		await expect(toggle).toHaveAttribute('aria-label', /^Theme: light\./)
		expect(await isDark(page)).toBe(false)

		await toggle.click()
		await expect(toggle).toHaveAttribute('aria-label', /^Theme: dark\./)
		await expect.poll(() => isDark(page)).toBe(true)

		await toggle.click()
		await expect(toggle).toHaveAttribute('aria-label', /^Theme: system\./)
		await expect.poll(() => isDark(page)).toBe(false)
	})

	test('chosen theme persists in localStorage across reload and route change', async ({ page }) => {
		// Scenario: pick dark, then reload and navigate in-app; the choice and storage value survive.
		await page.goto('.')
		await sel.themeToggle(page).click()
		await sel.themeToggle(page).click()
		await expect(sel.themeToggle(page)).toHaveAttribute('aria-label', /^Theme: dark\./)
		expect(await stored(page)).toBe('dark')

		await page.reload()
		await expect(sel.themeToggle(page)).toHaveAttribute('aria-label', /^Theme: dark\./)
		await expect.poll(() => isDark(page)).toBe(true)

		await page.getByRole('link', { name: 'Data', exact: true }).first().click()
		await expect(page).toHaveURL(/\/_studio\/data/)
		await expect(sel.themeToggle(page)).toHaveAttribute('aria-label', /^Theme: dark\./)
		expect(await isDark(page)).toBe(true)
		expect(await stored(page)).toBe('dark')
	})
})

test.describe('theme with dark OS scheme', () => {
	test.use({ colorScheme: 'dark' })

	test('system mode follows prefers-color-scheme: dark', async ({ page }) => {
		// Scenario: no stored preference means system; a dark OS renders the dark class.
		await page.goto('.')
		await expect(sel.themeToggle(page)).toHaveAttribute('aria-label', /^Theme: system\./)
		await expect.poll(() => isDark(page)).toBe(true)
	})
})
