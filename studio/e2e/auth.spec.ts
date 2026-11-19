import { expect, test } from './support/fixtures'
import type { Page } from '@playwright/test'

import { STACK_MODE } from './support/env'
import { signToken } from './support/jwt'
import { sel } from './support/selectors'

// Token dialog behaviour against the auth instance (auth.enabled, jwt.default=false).
test.skip(!STACK_MODE, 'Stack mode only (STUDIO_E2E_STACK=1).')

const DATA = 'data?db=prest_auth&schema=public&table=items'
const UNAUTHORIZED = 'Unauthorized – set or update your bearer token.'

async function saveToken(page: Page, token: string, remember = false) {
	await sel.tokenButton(page).click()
	await sel.tokenInput(page).fill(token)
	if (remember) await sel.rememberToken(page).check()
	await sel.saveToken(page).click()
}

const stored = (page: Page) =>
	page.evaluate(() => ({
		session: sessionStorage.getItem('prest.studio.token'),
		local: localStorage.getItem('prest.studio.token'),
	}))

test.describe('bearer token dialog', () => {
	// Scenario: an unauthenticated visitor sees Unauthorized on Overview and the catalog, never 'No tables found.'
	test('D6: without a token Overview and Data Explorer show Unauthorized', async ({
		authPage: page,
	}) => {
		await page.goto('./')
		await expect(page.getByText(/Unauthorized/).first()).toBeVisible()
		await page.goto(DATA)
		await expect(page.getByText(UNAUTHORIZED).first()).toBeVisible()
		await expect(page.getByText('No tables found.')).toHaveCount(0)
	})

	// Scenario: saving a token refetches the catalog without Refresh; /tables and /_mcp carry the Bearer header,
	// while /_studio/api/meta, /_health and /_ready do not.
	test('D5: Save token refetches without Refresh and requests carry the header', async ({
		authPage: page,
	}) => {
		const token = signToken()
		const seen: { path: string; auth: string | undefined }[] = []
		page.on('request', (req) => {
			seen.push({ path: new URL(req.url()).pathname, auth: req.headers()['authorization'] })
		})

		await page.goto(DATA)
		await expect(page.getByText(UNAUTHORIZED).first()).toBeVisible()
		const tablesWithToken = page.waitForRequest(
			(r) =>
				new URL(r.url()).pathname === '/tables' &&
				r.headers()['authorization'] === `Bearer ${token}`,
		)
		await saveToken(page, token)
		await tablesWithToken
		await expect(sel.treeTable(page, 'items')).toBeVisible()
		await expect(page.getByText(UNAUTHORIZED)).toHaveCount(0)

		// Overview hits the unauthenticated endpoints too; MCP connect hits /_mcp.
		await page.getByRole('link', { name: 'Overview' }).first().click()
		await expect(page.getByText('Bearer token')).toBeVisible()
		const mcpWithToken = page.waitForRequest(
			(r) =>
				new URL(r.url()).pathname === '/_mcp' && r.headers()['authorization'] === `Bearer ${token}`,
		)
		await page.getByRole('link', { name: 'MCP Explorer' }).first().click()
		await sel.mcpConnect(page).click()
		await mcpWithToken
		await expect(page.getByText('connected', { exact: true })).toBeVisible()

		const withHeader = new Set(seen.filter((r) => r.auth === `Bearer ${token}`).map((r) => r.path))
		expect(withHeader.has('/tables')).toBe(true)
		expect(withHeader.has('/_mcp')).toBe(true)
		for (const open of ['/_studio/api/meta', '/_health', '/_ready']) {
			const hits = seen.filter((r) => r.path === open)
			expect(hits.length, `${open} requested`).toBeGreaterThan(0)
			expect(
				hits.every((r) => r.auth === undefined),
				`${open} carries no Authorization`,
			).toBe(true)
		}
	})

	// Scenario: a token saved without 'Remember' stays out of both storages and is gone after reload.
	test('memory mode leaves sessionStorage and localStorage without the token', async ({
		authPage: page,
	}) => {
		await page.goto(DATA)
		await saveToken(page, signToken())
		await expect(sel.tokenButton(page)).not.toContainText('Set token')
		expect(await stored(page)).toEqual({ session: null, local: null })
		await page.reload()
		await expect(sel.tokenButton(page)).toContainText('Set token')
		await expect(page.getByText(UNAUTHORIZED).first()).toBeVisible()
	})

	// Scenario: 'Remember' persists to sessionStorage only; it survives reload but not a fresh tab.
	test('"Remember for this tab" writes sessionStorage only, survives reload, absent in a new page', async ({
		authPage: page,
	}) => {
		const token = signToken()
		await page.goto(DATA)
		await saveToken(page, token, true)
		await expect(sel.tokenButton(page)).not.toContainText('Set token')
		expect(await stored(page)).toEqual({ session: token, local: null })

		await page.reload()
		await expect(sel.treeTable(page, 'items')).toBeVisible()
		expect((await stored(page)).session).toBe(token)

		const other = await page.context().newPage()
		await other.goto(DATA)
		await expect(other.getByText(UNAUTHORIZED).first()).toBeVisible()
		expect((await stored(other)).session).toBeNull()
	})

	// Scenario: clearing a saved token removes it from storage and the catalog goes back to Unauthorized.
	test('D5: Clear token removes it and refetches back to Unauthorized', async ({
		authPage: page,
	}) => {
		await page.goto(DATA)
		await saveToken(page, signToken(), true)
		await page.reload()
		await expect(sel.treeTable(page, 'items')).toBeVisible()

		await sel.tokenButton(page).click()
		await sel.clearToken(page).click()
		await expect(page.getByText(UNAUTHORIZED).first()).toBeVisible()
		await expect(sel.treeTable(page, 'items')).toHaveCount(0)
		expect(await stored(page)).toEqual({ session: null, local: null })
	})
})
