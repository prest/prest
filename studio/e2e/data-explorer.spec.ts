import type { Page } from '@playwright/test'

import { MAIN_ORIGIN } from './support/env'
import { clipboardText, expect, test } from './support/fixtures'
import { sel } from './support/selectors'

const ITEMS_URL = 'data?db=prest&schema=public&table=items&page=1'

/** The rows table (the one without the Structure "Column" header). */
const rowsTable = (page: Page) =>
	page
		.locator('table')
		.filter({ hasNot: page.getByRole('columnheader', { name: 'Column', exact: true }) })
const structureTable = (page: Page) =>
	page
		.locator('table')
		.filter({ has: page.getByRole('columnheader', { name: 'Column', exact: true }) })
const bodyRows = (page: Page) => rowsTable(page).locator('tbody tr')
const firstCell = (page: Page) => bodyRows(page).first().locator('td').first()

test.describe('Data Explorer', () => {
	test('tree lists schemas, expands and collapses', async ({ page }) => {
		// Both fixture schemas appear as toggle buttons; clicking flips aria-expanded and hides tables.
		await page.goto('data')
		const publicBranch = page.locator('button[aria-expanded]').filter({ hasText: 'public' })
		const shopBranch = page.locator('button[aria-expanded]').filter({ hasText: 'shop' })
		await expect(publicBranch).toHaveAttribute('aria-expanded', 'true')
		await expect(shopBranch).toHaveAttribute('aria-expanded', 'true')
		await expect(sel.treeTable(page, 'orders')).toBeVisible()

		await shopBranch.click()
		await expect(shopBranch).toHaveAttribute('aria-expanded', 'false')
		await expect(sel.treeTable(page, 'orders')).toHaveCount(0)

		await shopBranch.click()
		await expect(shopBranch).toHaveAttribute('aria-expanded', 'true')
		await expect(sel.treeTable(page, 'orders')).toBeVisible()
	})

	test('tree hides sequences and keeps views (D8)', async ({ page }) => {
		// Regression D8: sequences (items_id_seq, ...) must not be listed; the view active_items must be.
		await page.goto('data')
		await expect(sel.treeTable(page, 'items')).toBeVisible()
		await expect(sel.treeTable(page, 'active_items')).toBeVisible()
		for (const seq of ['items_id_seq', 'orders_id_seq', 'Mixed Case_id_seq']) {
			await expect(sel.treeTable(page, seq)).toHaveCount(0)
		}
	})

	test('selecting a table writes the route search params', async ({ page }) => {
		// Clicking `items` deep-links db/schema/table/page into the URL.
		await page.goto('data')
		await sel.treeTable(page, 'items').click()
		await expect(page).toHaveURL((url) => {
			const p = url.searchParams
			return (
				url.pathname.endsWith('/data') &&
				p.get('db') === 'prest' &&
				p.get('schema') === 'public' &&
				p.get('table') === 'items' &&
				p.get('page') === '1'
			)
		})
		await expect(page.getByText('public.items')).toBeVisible()
	})

	test('structure lists the nine items columns', async ({ page }) => {
		// /show returns one structure row per column of public.items.
		await page.goto(ITEMS_URL)
		const names = structureTable(page).locator('tbody tr td:first-child')
		await expect(names).toHaveCount(9)
		await expect(names).toContainText([
			'id',
			'name',
			'qty',
			'price',
			'active',
			'created',
			'meta',
			'tags',
			'note',
		])
	})

	test('rows header follows table column order (D10)', async ({ page }) => {
		// Regression D10: JSON key order from the server is alphabetical; header must use table order.
		await page.goto(ITEMS_URL)
		await expect(rowsTable(page).locator('thead th')).toHaveText([
			'id',
			'name',
			'qty',
			'price',
			'active',
			'created',
			'meta',
			'tags',
			'note',
		])
	})

	test('paginates 25 / 25 / 23 rows with Previous and Next', async ({ page }) => {
		// 73 rows at page size 25: Next -> first id 26; page 3 has 23 rows and Next is disabled.
		await page.goto(ITEMS_URL)
		const next = page.getByRole('button', { name: 'Next' })
		const prev = page.getByRole('button', { name: 'Previous' })
		await expect(bodyRows(page)).toHaveCount(25)
		await expect(firstCell(page)).toHaveText('1')
		await expect(prev).toBeDisabled()

		await next.click()
		await expect(page.getByText(/^Page 2/)).toBeVisible()
		await expect(firstCell(page)).toHaveText('26')

		await next.click()
		await expect(page.getByText(/^Page 3/)).toBeVisible()
		await expect(bodyRows(page)).toHaveCount(23)
		await expect(next).toBeDisabled()

		await prev.click()
		await expect(page.getByText(/^Page 2/)).toBeVisible()
		await expect(firstCell(page)).toHaveText('26')
	})

	test('deep link and reload restore table and page', async ({ page }) => {
		// The route search params fully describe the view; reload must land on the same rows.
		await page.goto('data?db=prest&schema=public&table=items&page=3')
		await expect(bodyRows(page)).toHaveCount(23)
		await page.reload()
		await expect(page.getByText(/^Page 3/)).toBeVisible()
		await expect(bodyRows(page)).toHaveCount(23)
		await expect(firstCell(page)).toHaveText('51')
	})

	test('filter builder: eq, null, and removal resets to page 1', async ({ page }) => {
		// id eq 3 -> 1 row; note null -> 14 rows (g%5=0 of 73); removing a filter returns to page 1 of 25 rows.
		await page.goto(ITEMS_URL)
		await expect(bodyRows(page)).toHaveCount(25)

		await sel.filterColumn(page).fill('id')
		await sel.filterValue(page).fill('3')
		await sel.addFilter(page).click()
		await expect(bodyRows(page)).toHaveCount(1)
		await expect(firstCell(page)).toHaveText('3')
		await sel.removeFilter(page).click()
		await expect(bodyRows(page)).toHaveCount(25)

		await sel.filterColumn(page).fill('note')
		await sel.filterOperator(page).selectOption('null')
		await sel.addFilter(page).click()
		await expect(bodyRows(page)).toHaveCount(14)

		await sel.removeFilter(page).click()
		await expect(bodyRows(page)).toHaveCount(25)
		await expect(page.getByText(/^Page 1/)).toBeVisible()
	})

	test('removing a filter from page 2 resets to page 1', async ({ page }) => {
		// Adding a filter then removing it must not leave the view on a stale page.
		await page.goto('data?db=prest&schema=public&table=items&page=2')
		await expect(firstCell(page)).toHaveText('26')
		await sel.filterColumn(page).fill('id')
		await sel.filterValue(page).fill('3')
		await sel.addFilter(page).click()
		await expect(page.getByText(/^Page 1/)).toBeVisible()
		await expect(bodyRows(page)).toHaveCount(1)
		await sel.removeFilter(page).click()
		await expect(page.getByText(/^Page 1/)).toBeVisible()
		await expect(firstCell(page)).toHaveText('1')
	})

	test('tsquery filter lists rows without error (D4)', async ({ page, request }) => {
		// Regression D4: tsquery must send `name:tsquery=item`, not a `$tsquery.` value.
		const api = await request.get(`${MAIN_ORIGIN}/prest/public/items?name:tsquery=item`)
		expect(api.status()).toBe(200)
		expect(((await api.json()) as unknown[]).length).toBeGreaterThanOrEqual(1)

		await page.goto(ITEMS_URL)
		const sent = page.waitForRequest((req) => req.url().includes('name%3Atsquery=item'), {
			timeout: 5_000,
		})
		await sel.filterColumn(page).fill('name')
		await sel.filterOperator(page).selectOption('tsquery')
		await sel.filterValue(page).fill('item')
		await sel.addFilter(page).click()
		await sent
		await expect(bodyRows(page).first()).toBeVisible()
		expect(await bodyRows(page).count()).toBeGreaterThanOrEqual(1)
		await expect(page.locator('.text-destructive')).toHaveCount(0)
	})

	test('Copy URL puts an absolute URL on the clipboard (D1)', async ({ page }) => {
		// Regression D1: page 2 copy must be `<origin>/prest/public/items?_page=2&_page_size=25`; display stays relative.
		await page.goto('data?db=prest&schema=public&table=items&page=2')
		await expect(firstCell(page)).toHaveText('26')
		await page.getByRole('button', { name: 'Copy URL' }).click()
		expect(await clipboardText(page)).toBe(
			`${MAIN_ORIGIN}/prest/public/items?_page=2&_page_size=25`,
		)
		await expect(
			page.locator('code').filter({ hasText: '/prest/public/items?_page=2' }),
		).toHaveText('/prest/public/items?_page=2&_page_size=25')
	})

	test('Copy curl starts with curl and carries the origin', async ({ page }) => {
		// The curl snippet must be self-contained: `curl` plus the absolute URL.
		await page.goto(ITEMS_URL)
		await expect(bodyRows(page)).toHaveCount(25)
		await page.getByRole('button', { name: 'Copy curl' }).click()
		const text = await clipboardText(page)
		expect(text.startsWith('curl')).toBe(true)
		expect(text).toContain(MAIN_ORIGIN)
	})

	test('Mixed Case table shows the server identifier error (D9)', async ({ page, request }) => {
		// Regression D9: prest answers 400 for identifiers with spaces; Studio shows `Bad request – <server error>`.
		const res = await request.get(`${MAIN_ORIGIN}/prest/public/Mixed%20Case`)
		expect(res.status()).toBe(400)
		const body = (await res.json()) as { error: string }
		expect(body.error).toBe('invalid identifier in path')

		await page.goto('data')
		await sel.treeTable(page, 'Mixed Case').click()
		await expect(page.getByText('Bad request – invalid identifier in path')).toBeVisible()
	})

	test('big numbers render without precision loss (D7)', async ({ page, request }) => {
		// Regression D7: bigint and numeric(30,10) cells must show the exact literals, not rounded floats.
		const res = await request.get(`${MAIN_ORIGIN}/prest/public/big`)
		const text = await res.text()
		expect(text).toContain('9007199254740993')
		expect(text).toContain('12345678901234567890.1234567891')

		await page.goto('data')
		await sel.treeTable(page, 'big').click()
		const row = bodyRows(page).first()
		await expect(row.getByRole('cell', { name: '9007199254740993', exact: true })).toBeVisible()
		await expect(
			row.getByRole('cell', { name: '12345678901234567890.1234567891', exact: true }),
		).toBeVisible()
	})

	test('Studio only issues GET requests to prestd', async ({ page }) => {
		// Scenario: browse tree -> table -> structure -> rows -> next page -> filter add/remove -> copy URL/curl.
		// Every request to the prestd origin must be a GET (Studio is read-only).
		const methods: string[] = []
		page.on('request', (req) => {
			if (req.url().startsWith(MAIN_ORIGIN))
				methods.push(`${req.method()} ${new URL(req.url()).pathname}`)
		})
		await page.goto('data')
		await sel.treeTable(page, 'items').click()
		await expect(structureTable(page).locator('tbody tr')).toHaveCount(9)
		await expect(bodyRows(page)).toHaveCount(25)
		await page.getByRole('button', { name: 'Next' }).click()
		await expect(firstCell(page)).toHaveText('26')
		await sel.filterColumn(page).fill('id')
		await sel.filterValue(page).fill('3')
		await sel.addFilter(page).click()
		await expect(bodyRows(page)).toHaveCount(1)
		await sel.removeFilter(page).click()
		await expect(bodyRows(page)).toHaveCount(25)
		await page.getByRole('button', { name: 'Copy URL' }).click()
		await page.getByRole('button', { name: 'Copy curl' }).click()

		expect(methods.some((m) => m.includes('/prest/public/items'))).toBe(true)
		expect(methods.filter((m) => !m.startsWith('GET '))).toEqual([])
	})
})
