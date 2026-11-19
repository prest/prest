// Routing and SPA fallback served by prestd under /_studio/ (D11 asset 404s, in-app Not Found).
import { expect, test } from './support/fixtures'

test.describe('routing', () => {
	test('deep link with query params renders data page 2 and survives reload', async ({ page }) => {
		// Scenario: prestd serves index.html for /_studio/data; the router reads db/schema/table/page from the URL.
		const url = '/_studio/data?db=prest&schema=public&table=items&page=2'
		const response = await page.goto(url)
		expect(response?.status()).toBe(200)
		await expect(page.getByText(/^Page 2 · 25 rows/)).toBeVisible()
		await page.reload()
		await expect(page.getByText(/^Page 2 · 25 rows/)).toBeVisible()
		expect(new URL(page.url()).searchParams.get('page')).toBe('2')
	})

	test('unknown SPA route returns HTTP 200 and the in-app Not Found', async ({ page }) => {
		// Scenario: extension-less unknown path falls back to index.html.
		const response = await page.goto('/_studio/nope')
		expect(response?.status()).toBe(200)
		expect(response?.headers()['content-type']).toContain('text/html')
		await expect(page.getByText('Not Found')).toBeVisible()
	})

	test('SPA fallback keeps HTTP 200 text/html for a query-bearing client route', async ({
		request,
	}) => {
		// Scenario: D11 contract, /_studio/data?x=1 is a client route.
		const res = await request.get('/_studio/data?x=1')
		expect(res.status()).toBe(200)
		expect(res.headers()['content-type']).toContain('text/html')
	})

	test('missing static assets return 404 instead of index.html (D11)', async ({ request }) => {
		// Scenario: a stale hashed asset or an extension-bearing path must not be masked by the SPA fallback.
		expect((await request.get('/_studio/assets/nope.js')).status()).toBe(404)
		expect((await request.get('/_studio/nope.png')).status()).toBe(404)
	})

	test('unknown /_studio/api path returns 404', async ({ request }) => {
		// Scenario: only /_studio/api/meta exists under the API prefix.
		expect((await request.get('/_studio/api/nope')).status()).toBe(404)
	})

	test('/_studio redirects to /_studio/', async ({ request, page }) => {
		// Scenario: the bare prefix is redirected so relative asset URLs resolve.
		const res = await request.get('/_studio', { maxRedirects: 0 })
		expect([301, 302, 307, 308]).toContain(res.status())
		expect(res.headers()['location']).toMatch(/\/_studio\/$/)
		await page.goto('/_studio')
		await expect(page).toHaveURL(/\/_studio\/$/)
	})
})
