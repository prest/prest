import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'

import { clipboardText, test } from './support/fixtures'
import { MAIN_ORIGIN, STACK_MODE } from './support/env'
import { sel } from './support/selectors'

// MCP Explorer against the open main instance: connect, search, invoke (form and raw), history, AI-client panel.
test.skip(!STACK_MODE, 'Stack mode only (STUDIO_E2E_STACK=1).')

const MCP_HEADERS = {
	'content-type': 'application/json',
	accept: 'application/json, text/event-stream',
}

const toolButton = (page: Page, name: string) =>
	page.getByRole('button', { name: new RegExp(`^${name.replace('.', '\\.')}(\\s|$)`) })
const resultPanel = (page: Page) => page.locator('pre.max-h-72')

async function connect(page: Page) {
	await page.goto('mcp')
	await sel.mcpConnect(page).click()
	await expect(page.getByText('connected', { exact: true })).toBeVisible()
}

async function selectTool(page: Page, name: string) {
	await toolButton(page, name).click()
	await expect(page.locator('.font-mono.text-sm', { hasText: name })).toBeVisible()
}

test.describe('MCP Explorer', () => {
	// Scenario: Connect reports the negotiated protocol version and lists the server's tools.
	test('Connect shows protocol 2025-06-18 and lists tools including prest.select_table', async ({
		page,
	}) => {
		await connect(page)
		await expect(page.getByText('2025-06-18')).toBeVisible()
		await expect(toolButton(page, 'prest.select_table')).toBeVisible()
		await expect(toolButton(page, 'prest.list_tables')).toBeVisible()
	})

	// Scenario: the tool search matches names and, case-insensitively, descriptions.
	test('search filters tools by name and by description', async ({ page, request }) => {
		await connect(page)
		const list = await request.post(`${MAIN_ORIGIN}/_mcp`, {
			headers: MCP_HEADERS,
			data: { jsonrpc: '2.0', id: 1, method: 'tools/list' },
		})
		const tools = (await list.json()).result.tools as { name: string; description?: string }[]
		const all = page.locator('ul li button')
		await expect(all).toHaveCount(tools.length)

		await sel.mcpSearch(page).fill('select_table')
		await expect(all).toHaveCount(tools.filter((t) => t.name.includes('select_table')).length)
		await expect(toolButton(page, 'prest.select_table')).toBeVisible()
		await expect(toolButton(page, 'prest.list_tables')).toHaveCount(0)

		// A word that appears in a description but in no tool name.
		const target = tools.find((t) => t.name === 'prest.list_tables') ?? tools[0]
		const word = (target.description ?? '')
			.split(/\W+/)
			.find(
				(w) => w.length > 5 && !tools.some((t) => t.name.toLowerCase().includes(w.toLowerCase())),
			)
		expect(word, 'a description-only word').toBeTruthy()
		await sel.mcpSearch(page).fill(word!.toUpperCase())
		const expected = tools.filter((t) =>
			(t.description ?? '').toLowerCase().includes(word!.toLowerCase()),
		)
		await expect(all).toHaveCount(expected.length)
		await expect(toolButton(page, target.name)).toBeVisible()
	})

	// Scenario: a no-argument tool invoked from the form succeeds and lists tables.
	test('form invoke of prest.list_tables succeeds', async ({ page }) => {
		await connect(page)
		await selectTool(page, 'prest.list_tables')
		await sel.mcpInvoke(page).click()
		await expect(page.getByText('success', { exact: true })).toBeVisible()
		await expect(resultPanel(page)).toContainText('items')
	})

	// Scenario: Raw JSON mode sends the arguments verbatim and returns the filtered row.
	test('Raw JSON invoke of prest.select_table returns id 5', async ({ page }) => {
		await connect(page)
		await selectTool(page, 'prest.select_table')
		await sel.mcpRawToggle(page).click()
		await page.locator('textarea').fill(
			JSON.stringify({
				database: 'prest',
				schema: 'public',
				table: 'items',
				columns: ['id'],
				filters: { id: 5 },
			}),
		)
		await sel.mcpInvoke(page).click()
		await expect(page.getByText('success', { exact: true })).toBeVisible()
		await expect(resultPanel(page)).toContainText(/"id":\s*5\b/)
	})

	// Scenario: malformed JSON in Raw mode is reported in the result panel without calling the server.
	test('Raw JSON with invalid JSON shows the parse error', async ({ page }) => {
		await connect(page)
		await selectTool(page, 'prest.select_table')
		await sel.mcpRawToggle(page).click()
		await page.locator('textarea').fill('{not json')
		await sel.mcpInvoke(page).click()
		await expect(page.getByText('error', { exact: true })).toBeVisible()
		await expect(resultPanel(page)).toContainText(/JSON|Unexpected|Expected/i)
	})

	// D2: form coercion for array/object/limit fields.
	// Scenario: an integer limit and boolean filter from the form are honoured: exactly 2 active rows.
	test('D2: form invoke with filters {"active": true} and limit 2 returns exactly 2 rows', async ({
		page,
	}) => {
		await connect(page)
		await selectTool(page, 'prest.select_table')
		await sel.mcpArg(page, 'database').fill('prest')
		await sel.mcpArg(page, 'schema').fill('public')
		await sel.mcpArg(page, 'table').fill('items')
		await sel.mcpArg(page, 'columns').fill('id,name')
		await sel.mcpArg(page, 'filters').fill('{"active": true}')
		await sel.mcpArg(page, 'limit').fill('2')
		await sel.mcpInvoke(page).click()
		await expect(page.getByText('success', { exact: true })).toBeVisible()
		await expect(resultPanel(page)).toContainText('"id"')
		const text = (await resultPanel(page).textContent()) ?? ''
		expect(text.match(/"id":/g)).toHaveLength(2)
		await expect(resultPanel(page)).toContainText(/"count":\s*2\b/)
	})

	// Scenario: form text is coerced: columns split on commas, filters parsed as an object.
	test('D2: form invoke with columns "id,name", filters {"id": 3}, limit 2 returns id 3', async ({
		page,
	}) => {
		await connect(page)
		await selectTool(page, 'prest.select_table')
		await sel.mcpArg(page, 'database').fill('prest')
		await sel.mcpArg(page, 'schema').fill('public')
		await sel.mcpArg(page, 'table').fill('items')
		await sel.mcpArg(page, 'columns').fill('id,name')
		await sel.mcpArg(page, 'filters').fill('{"id": 3}')
		await sel.mcpArg(page, 'limit').fill('2')
		await sel.mcpInvoke(page).click()
		await expect(page.getByText('success', { exact: true })).toBeVisible()
		const out = resultPanel(page)
		await expect(out).toContainText(/"id":\s*3\b/)
		await expect(out).toContainText('item-3')
		await expect(out).not.toContainText(/"id":\s*4\b/)
	})

	// Scenario: a non-object value in an object field is rejected client-side.
	test('D2: invalid object field shows Field "filters" must be a JSON object.', async ({
		page,
	}) => {
		await connect(page)
		await selectTool(page, 'prest.select_table')
		await sel.mcpArg(page, 'database').fill('prest')
		await sel.mcpArg(page, 'schema').fill('public')
		await sel.mcpArg(page, 'table').fill('items')
		await sel.mcpArg(page, 'filters').fill('not-an-object')
		await sel.mcpInvoke(page).click()
		await expect(resultPanel(page)).toHaveText('Field "filters" must be a JSON object.')
	})

	// Scenario: an array field starting with '[' must parse as JSON.
	test('D2: array field starting with [ that fails to parse shows must be a JSON array.', async ({
		page,
	}) => {
		await connect(page)
		await selectTool(page, 'prest.select_table')
		await sel.mcpArg(page, 'database').fill('prest')
		await sel.mcpArg(page, 'schema').fill('public')
		await sel.mcpArg(page, 'table').fill('items')
		await sel.mcpArg(page, 'columns').fill('[bad')
		await sel.mcpInvoke(page).click()
		await expect(resultPanel(page)).toHaveText('Field "columns" must be a JSON array.')
	})

	// Scenario: a non-integer value in an integer field is rejected client-side.
	test('D2: integer field rejects non-integer input with must be an integer.', async ({ page }) => {
		await connect(page)
		await selectTool(page, 'prest.select_table')
		await sel.mcpArg(page, 'database').fill('prest')
		await sel.mcpArg(page, 'schema').fill('public')
		await sel.mcpArg(page, 'table').fill('items')
		await sel.mcpArg(page, 'limit').fill('abc')
		await sel.mcpInvoke(page).click()
		await expect(resultPanel(page)).toHaveText('Field "limit" must be an integer.')
		await sel.mcpArg(page, 'limit').fill('1.5')
		await sel.mcpInvoke(page).click()
		await expect(resultPanel(page)).toHaveText('Field "limit" must be an integer.')
	})

	// D3: server JSON-RPC message surfaced on a non-2xx response.
	// Scenario: a JSON-RPC error on a 400 response surfaces the server's message, not a transport error.
	test('D3: Raw JSON with table "nope" shows the server message, not MCP transport error', async ({
		page,
		request,
	}) => {
		const res = await request.post(`${MAIN_ORIGIN}/_mcp`, {
			headers: MCP_HEADERS,
			data: {
				jsonrpc: '2.0',
				id: 1,
				method: 'tools/call',
				params: {
					name: 'prest.select_table',
					arguments: { database: 'prest', schema: 'public', table: 'nope' },
				},
			},
		})
		const body = await res.json()
		const serverMessage: string = body.error?.message ?? body.result?.content?.[0]?.text
		expect(serverMessage, 'server reports a message for a missing table').toBeTruthy()

		await connect(page)
		await selectTool(page, 'prest.select_table')
		await sel.mcpRawToggle(page).click()
		await page
			.locator('textarea')
			.fill(JSON.stringify({ database: 'prest', schema: 'public', table: 'nope' }))
		await sel.mcpInvoke(page).click()
		await expect(page.getByText('error', { exact: true }).first()).toBeVisible()
		await expect(resultPanel(page)).not.toContainText('MCP transport error')
		await expect(resultPanel(page)).toContainText(serverMessage.slice(0, 40))
	})

	// Scenario: invocations are listed newest first and the list survives Reconnect.
	test('history lists newest first and survives Reconnect', async ({ page }) => {
		await connect(page)
		await selectTool(page, 'prest.list_tables')
		await sel.mcpInvoke(page).click()
		await expect(page.getByText('success', { exact: true })).toBeVisible()
		await selectTool(page, 'prest.select_table')
		await sel.mcpRawToggle(page).click()
		await page
			.locator('textarea')
			.fill(JSON.stringify({ database: 'prest', schema: 'public', table: 'items', limit: 1 }))
		await sel.mcpInvoke(page).click()
		await expect(resultPanel(page)).toContainText('"id"')

		const names = () => page.locator('div.rounded-md.border.p-2 span.font-mono').allTextContents()
		await expect.poll(names).toEqual(['prest.select_table', 'prest.list_tables'])

		await sel
			.mcpConnect(page)
			.or(page.getByRole('button', { name: 'Reconnect' }))
			.click()
		await expect(page.getByText('connected', { exact: true })).toBeVisible()
		await expect.poll(names).toEqual(['prest.select_table', 'prest.list_tables'])
	})

	// Scenario: the AI-client panel shows the endpoint URL, a config with a token placeholder, and copies the URL.
	test('AI client panel shows <origin>/_mcp and a config with <YOUR_TOKEN>', async ({ page }) => {
		await page.goto('mcp')
		const url = `${MAIN_ORIGIN}/_mcp`
		await expect(page.locator('code', { hasText: url })).toHaveText(url)
		const config = page.locator('pre', { hasText: 'mcpServers' })
		await expect(config).toContainText('<YOUR_TOKEN>')
		await expect(config).toContainText(`"url": "${url}"`)
		await page.getByRole('button', { name: 'Copy URL' }).click()
		await expect.poll(() => clipboardText(page)).toBe(url)
	})
})
