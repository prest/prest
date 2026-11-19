import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const h = vi.hoisted(() => ({
	search: {} as Record<string, unknown>,
	listTables: vi.fn(),
	listDatabases: vi.fn(),
	showTable: vi.fn(),
	queryTable: vi.fn(),
	absoluteUrl: vi.fn(),
}))

vi.mock('@tanstack/react-router', () => ({
	getRouteApi: () => ({ useSearch: () => h.search, useNavigate: () => vi.fn() }),
}))
vi.mock('@/app/providers', () => ({ usePrestClient: () => ({}) }))
vi.mock('@/lib/api/prest', async (orig) => ({
	...(await orig<typeof import('@/lib/api/prest')>()),
	listTables: h.listTables,
	listDatabases: h.listDatabases,
	showTable: h.showTable,
	queryTable: h.queryTable,
}))
vi.mock('@/lib/api/url', async (orig) => ({
	...(await orig<typeof import('@/lib/api/url')>()),
	absoluteUrl: h.absoluteUrl,
}))

import { DataExplorerPage } from './DataExplorerPage'

function renderPage() {
	const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
	return render(
		<QueryClientProvider client={qc}>
			<DataExplorerPage />
		</QueryClientProvider>,
	)
}

beforeEach(() => {
	vi.resetAllMocks()
	h.search = {}
	h.listDatabases.mockResolvedValue({ disabled: false, items: [{ datname: 'prest' }] })
	h.showTable.mockResolvedValue([])
	h.queryTable.mockResolvedValue({ rows: [], durationMs: 1 })
})

describe('DataExplorerPage catalog card', { concurrent: false }, () => {
	it('shows the error instead of "No tables found."', async () => {
		h.listTables.mockResolvedValue({
			disabled: false,
			error: 'Unauthorized – set or update your bearer token.',
		})
		renderPage()
		expect(
			await screen.findByText('Unauthorized – set or update your bearer token.'),
		).toBeInTheDocument()
		expect(screen.queryByText('No tables found.')).not.toBeInTheDocument()
	})

	it('shows the restricted message when listing is disabled', async () => {
		h.listTables.mockResolvedValue({ disabled: true })
		renderPage()
		expect(await screen.findByText('Table listing is restricted.')).toBeInTheDocument()
	})

	it('hides sequences and keeps views in the tree', async () => {
		h.listTables.mockResolvedValue({
			disabled: false,
			items: [
				{ name: 'items', schema: 'public', type: 'table' },
				{ name: 'items_id_seq', schema: 'public', type: 'sequence' },
				{ name: 'active_items', schema: 'public', type: 'view' },
			],
		})
		renderPage()
		await screen.findByText('public')
		expect(await screen.findByText('active_items')).toBeInTheDocument()
		expect(screen.queryByText('items_id_seq')).not.toBeInTheDocument()
	})

	it('shows the databases error under the selector', async () => {
		h.listDatabases.mockResolvedValue({ disabled: false, error: 'db listing failed' })
		h.listTables.mockResolvedValue({ disabled: false, items: [] })
		renderPage()
		expect(await screen.findByText('db listing failed')).toBeInTheDocument()
	})
})

describe('DataExplorerPage selected table', { concurrent: false }, () => {
	beforeEach(() => {
		h.search = { db: 'prest', schema: 'public', table: 'items', page: 2 }
		h.listTables.mockResolvedValue({ disabled: false, items: [] })
		h.absoluteUrl.mockReturnValue('http://studio.test/prest/public/items?_page=2&_page_size=25')
	})

	it('orders columns by table structure', async () => {
		h.showTable.mockResolvedValue([
			{ column_name: 'id', position: 1 },
			{ column_name: 'name', position: 2 },
			{ column_name: 'qty', position: 3 },
		])
		h.queryTable.mockResolvedValue({ rows: [{ qty: 1, name: 'a', id: 2 }], durationMs: 1 })
		renderPage()
		const headers = await screen.findAllByRole('columnheader')
		await waitFor(() => {
			const names = screen.getAllByRole('columnheader').map((c) => c.textContent)
			expect(names.slice(-3)).toEqual(['id', 'name', 'qty'])
		})
		expect(headers.length).toBeGreaterThan(0)
	})

	it('builds the copied URL through absoluteUrl(fullUrl, origin)', async () => {
		const writeText = vi.fn().mockResolvedValue(undefined)
		Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
		renderPage()
		await userEvent.click(await screen.findByRole('button', { name: 'Copy URL' }))
		expect(h.absoluteUrl).toHaveBeenCalledWith(
			'/prest/public/items?_page=2&_page_size=25',
			window.location.origin,
		)
		expect(writeText).toHaveBeenCalledWith(
			'http://studio.test/prest/public/items?_page=2&_page_size=25',
		)
		// displayed URL stays relative
		expect(screen.getByText('/prest/public/items?_page=2&_page_size=25')).toBeInTheDocument()
	})
})
