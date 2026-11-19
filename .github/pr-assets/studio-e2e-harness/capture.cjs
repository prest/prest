// Before/after defect screenshots. Usage: NODE_PATH=studio/node_modules node capture.cjs <outdir>
const { chromium } = require('@playwright/test')

const OUT = process.argv[2]
const SIDES = {
	before: { main: 'http://localhost:39981', auth: 'http://localhost:39982', tag: 'BEFORE · main @ 508d79e' },
	after: { main: 'http://localhost:39991', auth: 'http://localhost:39992', tag: 'AFTER · test/studio-e2e-harness' },
}

async function banner(page, side, text) {
	await page.evaluate(
		([tag, text, before]) => {
			document.getElementById('__proof')?.remove()
			const d = document.createElement('div')
			d.id = '__proof'
			d.style.cssText = `font:600 14px/1.4 ui-monospace,monospace;padding:8px 12px;color:#fff;background:${before ? '#b42318' : '#067647'};white-space:pre-wrap;word-break:break-all`
			d.textContent = `${tag} — ${text}`
			document.body.prepend(d)
		},
		[SIDES[side].tag, text, side === 'before'],
	)
}

const settle = (page) => page.waitForTimeout(1200)

const cases = {
	async d1(page, s, side) {
		await page.goto(`${s.main}/_studio/data?db=prest&schema=public&table=items&page=2`)
		await page.getByRole('button', { name: 'Copy URL' }).click()
		await settle(page)
		const clip = await page.evaluate(() => navigator.clipboard.readText())
		await banner(page, side, `D1 Copy URL → clipboard: "${clip}"`)
	},
	async d2(page, s, side) {
		await mcpTool(page, s)
		for (const [k, v] of Object.entries({ database: 'prest', schema: 'public', table: 'items', columns: 'id,name', filters: '{"id": 3}', limit: '2' }))
			await page.locator(`#arg-${k}`).fill(v)
		await page.getByRole('button', { name: 'Invoke tool' }).click()
		await settle(page)
		await banner(page, side, 'D2 MCP form: columns "id,name", filters {"id": 3}, limit 2')
	},
	async d3(page, s, side) {
		await mcpTool(page, s)
		await page.getByRole('button', { name: 'Raw JSON' }).click()
		await page.locator('textarea').fill(JSON.stringify({ database: 'prest', schema: 'public', table: 'nope' }))
		await page.getByRole('button', { name: 'Invoke tool' }).click()
		await settle(page)
		await banner(page, side, 'D3 MCP raw invoke on missing table "nope": server error message shown?')
	},
	async d4(page, s, side) {
		await page.goto(`${s.main}/_studio/data?db=prest&schema=public&table=items&page=1`)
		await page.getByPlaceholder('column').fill('name')
		await page.locator('select').nth(1).selectOption('tsquery')
		await page.getByPlaceholder('value').fill('item')
		await page.getByRole('button', { name: 'Add filter' }).click()
		await settle(page)
		await banner(page, side, 'D4 filter name tsquery "item"')
	},
	async d6(page, s, side) {
		await page.goto(`${s.auth}/_studio/data`)
		await settle(page)
		await banner(page, side, 'D6 JWT instance, no token: Data Explorer table list')
	},
	async d7(page, s, side) {
		await page.goto(`${s.main}/_studio/data?db=prest&schema=public&table=big&page=1`)
		await settle(page)
		await banner(page, side, 'D7 public.big: DB has 9007199254740993 and 12345678901234567890.1234567891')
	},
	async d8(page, s, side) {
		await page.goto(`${s.main}/_studio/`)
		await settle(page)
		await banner(page, side, 'D8 Overview counts (fixture: 2 schemas, 6 tables/views) + sequences')
	},
	async d8tree(page, s, side) {
		await page.goto(`${s.main}/_studio/data`)
		await settle(page)
		await banner(page, side, 'D8 Data Explorer tree: sequences (*_id_seq) listed as tables?')
	},
	async d9(page, s, side) {
		await page.goto(`${s.main}/_studio/data`)
		await page.getByRole('button', { name: 'Mixed Case', exact: true }).click()
		await settle(page)
		await banner(page, side, 'D9 table "Mixed Case" (API answers 400 invalid identifier)')
	},
	async d10(page, s, side) {
		await page.goto(`${s.main}/_studio/data?db=prest&schema=public&table=items&page=1`)
		await settle(page)
		await banner(page, side, 'D10 rows header order (table order: id,name,qty,price,active,created,meta,tags,note)')
	},
	async d11(page, s, side) {
		const res = await page.goto(`${s.main}/_studio/assets/nope.js`)
		await settle(page)
		await banner(page, side, `D11 GET /_studio/assets/nope.js → HTTP ${res.status()} ${res.headers()['content-type']}`)
	},
	async restricted(page, s, side) {
		await page.route('**/tables', (r) => r.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"unauthorized listing"}' }))
		await page.goto(`${s.main}/_studio/`)
		await settle(page)
		await banner(page, side, 'Restricted listing: /tables → 401 {"error":"unauthorized listing"}')
	},
}

async function mcpTool(page, s) {
	await page.goto(`${s.main}/_studio/mcp`)
	await page.getByRole('button', { name: 'Connect', exact: true }).click()
	await page.getByText('connected', { exact: true }).waitFor()
	await page.getByRole('button', { name: /^prest\.select_table(\s|$)/ }).click()
}

;(async () => {
	const browser = await chromium.launch()
	for (const side of Object.keys(SIDES)) {
		const s = SIDES[side]
		for (const [name, fn] of Object.entries(cases)) {
			const ctx = await browser.newContext({ viewport: { width: 1280, height: 860 }, colorScheme: 'light' })
			await ctx.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: s.main })
			const page = await ctx.newPage()
			try {
				await fn(page, s, side)
				await page.screenshot({ path: `${OUT}/${name}-${side}.png`, fullPage: true })
				console.log('ok', name, side)
			} catch (e) {
				console.log('FAIL', name, side, e.message.split('\n')[0])
			}
			await ctx.close()
		}
		// D13: mobile overflow, full page so extra width shows.
		const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 })
		const page = await ctx.newPage()
		for (const [name, path] of [['d13-overview', ''], ['d13-data', 'data?db=prest&schema=public&table=items&page=1']]) {
			await page.goto(`${s.main}/_studio/${path}`)
			await settle(page)
			const w = await page.evaluate(() => [document.documentElement.scrollWidth, document.documentElement.clientWidth])
			await banner(page, side, `D13 390px: scrollWidth ${w[0]} / clientWidth ${w[1]}`)
			await page.screenshot({ path: `${OUT}/${name}-${side}.png`, fullPage: true })
			console.log('ok', name, side, w)
		}
		await ctx.close()
	}
	await browser.close()
})()
