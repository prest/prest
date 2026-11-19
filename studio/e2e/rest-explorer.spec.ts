import { MAIN_ORIGIN } from './support/env'
import { clipboardText, expect, test } from './support/fixtures'
import { sel } from './support/selectors'

const ITEMS_PATH = '/prest/public/items'

test.describe('REST Explorer', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('rest')
	})

	test('sends a GET with query params and shows an HTTP 200 badge', async ({ page }) => {
		// _page=1 and _page_size=5 are sent; the response body is a JSON array of 5 items.
		await sel.restPath(page).fill(ITEMS_PATH)
		await sel.restKey(page).first().fill('_page')
		await sel.restValue(page).first().fill('1')
		await page.getByRole('button', { name: 'Add parameter' }).click()
		await sel.restKey(page).nth(1).fill('_page_size')
		await sel.restValue(page).nth(1).fill('5')
		await sel.send(page).click()

		await expect(page.getByText('HTTP 200')).toBeVisible()
		const pre = page.locator('pre')
		await expect(pre).toContainText('"name": "item-1"')
		const rows = JSON.parse(await pre.innerText()) as Array<{ id: number }>
		expect(rows).toHaveLength(5)
		expect(rows.map((r) => r.id)).toEqual([1, 2, 3, 4, 5])
	})

	test('a disabled parameter is not sent', async ({ page }) => {
		// Unchecking "Enable parameter" removes it from the displayed URL and from the request.
		await sel.restPath(page).fill(ITEMS_PATH)
		await sel.restKey(page).first().fill('_page_size')
		await sel.restValue(page).first().fill('2')
		await page.getByRole('checkbox', { name: 'Enable parameter' }).uncheck()
		await expect(page.locator('code').filter({ hasText: ITEMS_PATH })).toHaveText(ITEMS_PATH)

		const request = page.waitForRequest((req) => new URL(req.url()).pathname === ITEMS_PATH)
		await sel.send(page).click()
		expect((await request).url()).not.toContain('_page_size')
		await expect(page.getByText('HTTP 200')).toBeVisible()
	})

	test('an unknown table shows a red 404 badge with the body', async ({ page }) => {
		// The raw response is displayed even for error statuses; the badge is destructive, not success.
		await sel.restPath(page).fill('/prest/public/nope')
		await sel.send(page).click()
		const badge = page.getByText('HTTP 404')
		await expect(badge).toBeVisible()
		await expect(badge).toHaveClass(/destructive/)
		await expect(page.locator('pre')).not.toBeEmpty()
	})

	test('a cross-origin URL is blocked without sending a request', async ({ page }) => {
		// Absolute URLs on another origin are refused so the bearer token can never leak.
		const external: string[] = []
		page.on('request', (req) => {
			if (req.url().includes('example.invalid')) external.push(req.url())
		})
		await sel.restPath(page).fill('http://example.invalid/prest/public/items')
		await sel.send(page).click()
		await expect(page.getByText('Cross-origin requests are not allowed.')).toBeVisible()
		expect(external).toEqual([])
	})

	test('Copy URL puts an absolute URL on the clipboard (D1)', async ({ page }) => {
		// Regression D1: copied value is `<origin>/prest/public/items?_page=2`; the displayed URL stays relative.
		await sel.restPath(page).fill(ITEMS_PATH)
		await sel.restKey(page).first().fill('_page')
		await sel.restValue(page).first().fill('2')
		await expect(page.locator('code')).toHaveText(`${ITEMS_PATH}?_page=2`)
		await page.getByRole('button', { name: 'Copy URL' }).click()
		expect(await clipboardText(page)).toBe(`${MAIN_ORIGIN}${ITEMS_PATH}?_page=2`)
	})

	test('big numbers survive in the response text (D7)', async ({ page }) => {
		// Regression D7: the pretty-printed response must keep both literals exactly.
		await sel.restPath(page).fill('/prest/public/big')
		await sel.send(page).click()
		const pre = page.locator('pre')
		await expect(pre).toContainText('9007199254740993')
		await expect(pre).toContainText('12345678901234567890.1234567891')
	})
})
