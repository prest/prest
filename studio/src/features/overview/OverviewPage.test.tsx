import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const h = vi.hoisted(() => ({
	listDatabases: vi.fn(),
	listSchemas: vi.fn(),
	listTables: vi.fn(),
}))

vi.mock('@tanstack/react-router', () => ({
	Link: ({ children, to }: { children: React.ReactNode; to: string }) => (
		<a href={to}>{children}</a>
	),
}))
vi.mock('@/app/providers', () => ({
	usePrestClient: () => ({}),
	useAuth: () => ({ hasToken: false }),
}))
vi.mock('@/lib/api/prest', () => ({
	getMeta: vi.fn().mockResolvedValue({
		version: 'v1',
		commit: 'abcdef0123456789abcdef0123456789abcdef01',
		buildDate: 'd',
		apiBasePath: '/prest',
		mcpEndpoint: '/_mcp',
	}),
	getHealth: vi.fn().mockResolvedValue({ ok: true, status: 200 }),
	getReady: vi.fn().mockResolvedValue({ ok: true, status: 200 }),
	probeMcp: vi.fn().mockResolvedValue({ ok: true, status: 200 }),
	listDatabases: h.listDatabases,
	listSchemas: h.listSchemas,
	listTables: h.listTables,
	pickName: (await_: Record<string, unknown>, keys: string[]) => {
		for (const k of keys) if (typeof await_[k] === 'string' && await_[k]) return await_[k] as string
		for (const v of Object.values(await_)) if (typeof v === 'string' && v) return v
		return ''
	},
	NAME_KEYS: {
		database: ['datname', 'database', 'name', 'db'],
		schema: ['schema_name', 'schema', 'name', 'nspname'],
		table: ['table_name', 'table', 'name', 'relname'],
	},
}))

import { OverviewPage } from './OverviewPage'

function renderPage() {
	const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
	return render(
		<QueryClientProvider client={qc}>
			<OverviewPage />
		</QueryClientProvider>,
	)
}

beforeEach(() => {
	h.listDatabases.mockResolvedValue({ disabled: false, items: [{ datname: 'prest' }] })
	h.listSchemas.mockResolvedValue({
		disabled: false,
		items: ['public', 'shop', 'pg_catalog', 'information_schema', 'pg_toast'].map((s) => ({
			schema_name: s,
		})),
	})
	h.listTables.mockResolvedValue({
		disabled: false,
		items: [
			...['items', 'active_items', 'Mixed Case', 'TestCase', 'big'].map((name) => ({
				name,
				schema: 'public',
				type: name === 'active_items' ? 'view' : 'table',
			})),
			{ name: 'orders', schema: 'shop', type: 'table' },
			{ name: 'items_id_seq', schema: 'public', type: 'sequence' },
			{ name: 'orders_id_seq', schema: 'shop', type: 'sequence' },
			{ name: 'pg_class', schema: 'pg_catalog', type: 'table' },
		],
	})
})

describe('OverviewPage counts', { concurrent: false }, () => {
	it('counts non-system schemas and browsable relations (Schemas 2, Tables 6)', async () => {
		renderPage()
		const schemasLabel = await screen.findByText('Schemas')
		const tablesLabel = screen.getByText('Tables')
		await vi.waitFor(() => {
			expect(schemasLabel.previousElementSibling).toHaveTextContent('2')
			expect(tablesLabel.previousElementSibling).toHaveTextContent('6')
		})
	})

	it('shows a dash for restricted table listing', async () => {
		h.listTables.mockResolvedValue({ disabled: true })
		renderPage()
		const label = await screen.findByText('Tables (restricted)')
		expect(label.previousElementSibling).toHaveTextContent('—')
	})

	it('wraps long metadata values', async () => {
		renderPage()
		const commit = await screen.findByText('abcdef0123456789abcdef0123456789abcdef01')
		expect(commit).toHaveClass('break-all', 'min-w-0')
	})
})
