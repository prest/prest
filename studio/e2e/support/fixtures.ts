// Playwright `test` extended with an `authPage` bound to the auth instance, plus a clipboard helper.
import { test as base, type Page } from '@playwright/test'

import { AUTH_BASE_URL } from './env'

export const test = base.extend<{ authPage: Page }>({
	authPage: async ({ browser }, provide) => {
		const context = await browser.newContext({
			baseURL: AUTH_BASE_URL,
			permissions: ['clipboard-read', 'clipboard-write'],
		})
		const page = await context.newPage()
		await provide(page)
		await context.close()
	},
})

export { expect } from '@playwright/test'

export async function clipboardText(page: Page): Promise<string> {
	return page.evaluate(() => navigator.clipboard.readText())
}
