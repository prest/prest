import * as React from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import {
	ChevronDown,
	ChevronRight,
	Columns3,
	Lock,
	Plus,
	RefreshCw,
	Table2,
	Trash2,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { CopyButton } from '@/components/copy-button'
import { usePrestClient } from '@/app/providers'
import {
	listDatabases,
	listTables,
	pickName,
	queryTable,
	showTable,
	NAME_KEYS,
} from '@/lib/api/prest'
import {
	buildUrl,
	FILTER_OPS,
	UNARY_OPS,
	type Filter,
	type FilterOp,
	type QuerySpec,
} from '@/lib/api/url'
import { buildCurl } from '@/lib/api/curl'
import { toErrorMessage } from '@/lib/errors'
import type { Row } from '@/lib/api/types'
import { cn } from '@/lib/utils'

const PAGE_SIZE = 25
const routeApi = getRouteApi('/data')

interface TableNode {
	schema: string
	table: string
}

function detectSchema(record: Row): string {
	return pickName(record, [...NAME_KEYS.schema]) || 'public'
}

function groupBySchema(items: Row[]): Map<string, string[]> {
	const groups = new Map<string, string[]>()
	for (const record of items) {
		const table = pickName(record, [...NAME_KEYS.table])
		if (!table) continue
		const schema = detectSchema(record)
		const list = groups.get(schema) ?? []
		list.push(table)
		groups.set(schema, list)
	}
	for (const list of groups.values()) list.sort()
	return groups
}

export function DataExplorerPage() {
	const client = usePrestClient()
	const search = routeApi.useSearch()
	const navigate = routeApi.useNavigate()

	const [filters, setFilters] = React.useState<Filter[]>([])
	const [draft, setDraft] = React.useState<Filter>({ column: '', op: 'eq', value: '' })

	const databases = useQuery({ queryKey: ['databases'], queryFn: () => listDatabases(client) })
	const tables = useQuery({ queryKey: ['tables'], queryFn: () => listTables(client) })

	const dbOptions = React.useMemo(
		() =>
			(databases.data?.items ?? [])
				.map((r) => pickName(r, [...NAME_KEYS.database]))
				.filter(Boolean),
		[databases.data],
	)
	const selectedDb = search.db ?? dbOptions[0] ?? ''
	const groups = React.useMemo(() => groupBySchema(tables.data?.items ?? []), [tables.data])

	const selected: TableNode | null =
		search.schema && search.table ? { schema: search.schema, table: search.table } : null
	const page = search.page ?? 1

	const spec: QuerySpec = React.useMemo(
		() => ({ filters, page, pageSize: PAGE_SIZE }),
		[filters, page],
	)

	const dataPath =
		selectedDb && selected
			? `/${encodeURIComponent(selectedDb)}/${encodeURIComponent(selected.schema)}/${encodeURIComponent(selected.table)}`
			: ''

	const structure = useQuery({
		queryKey: ['structure', selectedDb, selected?.schema, selected?.table],
		queryFn: () => showTable(client, selectedDb, selected!.schema, selected!.table),
		enabled: Boolean(selectedDb && selected),
	})

	const rows = useQuery({
		queryKey: ['rows', selectedDb, selected?.schema, selected?.table, spec],
		queryFn: ({ signal }) =>
			queryTable(client, selectedDb, selected!.schema, selected!.table, spec, signal),
		enabled: Boolean(selectedDb && selected),
		placeholderData: keepPreviousData,
	})

	const selectTable = (schema: string, table: string) => {
		setFilters([])
		navigate({ search: (prev) => ({ ...prev, db: selectedDb, schema, table, page: 1 }) })
	}

	const setPage = (next: number) => {
		navigate({ search: (prev) => ({ ...prev, page: Math.max(1, next) }) })
	}

	const columns = React.useMemo<string[]>(() => {
		if (rows.data && rows.data.rows.length > 0) return Object.keys(rows.data.rows[0])
		return (structure.data ?? [])
			.map((c) => c.column_name)
			.filter((c): c is string => typeof c === 'string')
	}, [rows.data, structure.data])

	const fullUrl = dataPath ? buildUrl(dataPath, spec) : ''
	const curl = fullUrl ? buildCurl({ url: fullUrl, origin: window.location.origin }) : ''

	const applyDraft = () => {
		if (!draft.column) return
		setFilters((prev) => [...prev, draft])
		setDraft({ column: '', op: 'eq', value: '' })
		setPage(1)
	}

	return (
		<div className="flex flex-col gap-4">
			<div className="flex flex-wrap items-center justify-between gap-2">
				<div>
					<h1 className="text-2xl font-semibold">Data Explorer</h1>
					<p className="text-sm text-muted-foreground">Browse schemas, tables and rows.</p>
				</div>
				<Badge variant="warning" title="Studio issues GET requests only">
					<Lock className="size-3" /> read-only
				</Badge>
			</div>

			<div className="grid gap-4 lg:grid-cols-[16rem_1fr]">
				<Card className="h-fit">
					<CardHeader className="gap-2">
						<CardTitle className="text-sm">Catalog</CardTitle>
						<div className="flex flex-col gap-1">
							<Label htmlFor="db-select" className="text-xs text-muted-foreground">
								Database
							</Label>
							{dbOptions.length > 0 ? (
								<select
									id="db-select"
									className="h-9 rounded-md border border-input bg-background px-2 text-sm"
									value={selectedDb}
									onChange={(e) =>
										navigate({ search: (prev) => ({ ...prev, db: e.target.value }) })
									}
								>
									{dbOptions.map((db) => (
										<option key={db} value={db}>
											{db}
										</option>
									))}
								</select>
							) : (
								<Input
									id="db-select"
									placeholder="database name"
									defaultValue={selectedDb}
									onBlur={(e) =>
										navigate({ search: (prev) => ({ ...prev, db: e.target.value.trim() }) })
									}
								/>
							)}
						</div>
					</CardHeader>
					<CardContent className="max-h-[60vh] overflow-auto">
						{tables.data?.disabled ? (
							<p className="text-xs text-muted-foreground">Table listing is restricted.</p>
						) : tables.isLoading ? (
							<p className="text-xs text-muted-foreground">Loading…</p>
						) : groups.size === 0 ? (
							<p className="text-xs text-muted-foreground">No tables found.</p>
						) : (
							<SchemaTree groups={groups} selected={selected} onSelect={selectTable} />
						)}
					</CardContent>
				</Card>

				<div className="flex min-w-0 flex-col gap-4">
					{!selected ? (
						<Card>
							<CardContent className="p-8 text-center text-sm text-muted-foreground">
								Select a table from the catalog to view its structure and rows.
							</CardContent>
						</Card>
					) : (
						<>
							<Card>
								<CardHeader className="flex-row items-center justify-between gap-2 space-y-0">
									<CardTitle className="flex items-center gap-2 text-sm">
										<Table2 className="size-4" />
										<span className="font-mono">
											{selected.schema}.{selected.table}
										</span>
									</CardTitle>
									<div className="flex items-center gap-2">
										{fullUrl ? <CopyButton value={fullUrl} label="Copy URL" /> : null}
										{curl ? <CopyButton value={curl} label="Copy curl" /> : null}
										<Button
											variant="outline"
											size="sm"
											onClick={() => rows.refetch()}
											aria-label="Refresh rows"
										>
											<RefreshCw className={cn(rows.isFetching && 'animate-spin')} />
										</Button>
									</div>
								</CardHeader>
								<CardContent className="flex flex-col gap-3">
									<FilterBar
										columns={columns}
										filters={filters}
										draft={draft}
										setDraft={setDraft}
										onApply={applyDraft}
										onRemove={(i) => {
											setFilters((prev) => prev.filter((_, idx) => idx !== i))
											setPage(1)
										}}
									/>
									{fullUrl ? (
										<code className="block overflow-x-auto rounded-md bg-muted px-3 py-2 font-mono text-xs">
											{fullUrl}
										</code>
									) : null}
								</CardContent>
							</Card>

							{structure.data && structure.data.length > 0 ? (
								<Card>
									<CardHeader>
										<CardTitle className="flex items-center gap-2 text-sm">
											<Columns3 className="size-4" /> Structure
										</CardTitle>
									</CardHeader>
									<CardContent className="overflow-x-auto">
										<table className="w-full text-sm">
											<thead className="text-left text-xs text-muted-foreground">
												<tr>
													<th className="py-1 pr-4 font-medium">Column</th>
													<th className="py-1 pr-4 font-medium">Type</th>
													<th className="py-1 font-medium">Nullable</th>
												</tr>
											</thead>
											<tbody>
												{structure.data.map((col, i) => (
													<tr key={col.column_name ?? i} className="border-t border-border">
														<td className="py-1 pr-4 font-mono">{col.column_name ?? '—'}</td>
														<td className="py-1 pr-4 text-muted-foreground">
															{col.data_type ?? '—'}
														</td>
														<td className="py-1 text-muted-foreground">{col.is_nullable ?? '—'}</td>
													</tr>
												))}
											</tbody>
										</table>
									</CardContent>
								</Card>
							) : null}

							<Card>
								<CardContent className="p-0">
									{rows.isError ? (
										<p className="p-4 text-sm text-destructive">{toErrorMessage(rows.error)}</p>
									) : (
										<RowsTable
											columns={columns}
											rows={rows.data?.rows ?? []}
											loading={rows.isLoading}
										/>
									)}
								</CardContent>
							</Card>

							<div className="flex items-center justify-between text-sm">
								<span className="text-muted-foreground">
									Page {page}
									{rows.data ? ` · ${rows.data.rows.length} rows · ${rows.data.durationMs}ms` : ''}
								</span>
								<div className="flex gap-2">
									<Button
										variant="outline"
										size="sm"
										disabled={page <= 1}
										onClick={() => setPage(page - 1)}
									>
										Previous
									</Button>
									<Button
										variant="outline"
										size="sm"
										disabled={(rows.data?.rows.length ?? 0) < PAGE_SIZE}
										onClick={() => setPage(page + 1)}
									>
										Next
									</Button>
								</div>
							</div>
						</>
					)}
				</div>
			</div>
		</div>
	)
}

function SchemaTree({
	groups,
	selected,
	onSelect,
}: {
	groups: Map<string, string[]>
	selected: TableNode | null
	onSelect: (schema: string, table: string) => void
}) {
	return (
		<ul className="flex flex-col gap-0.5">
			{[...groups.entries()].map(([schema, tableNames]) => (
				<SchemaBranch
					key={schema}
					schema={schema}
					tables={tableNames}
					selected={selected}
					onSelect={onSelect}
				/>
			))}
		</ul>
	)
}

function SchemaBranch({
	schema,
	tables,
	selected,
	onSelect,
}: {
	schema: string
	tables: string[]
	selected: TableNode | null
	onSelect: (schema: string, table: string) => void
}) {
	const [open, setOpen] = React.useState(true)
	return (
		<li>
			<button
				type="button"
				onClick={() => setOpen((v) => !v)}
				className="flex w-full items-center gap-1 rounded-md px-1 py-1 text-sm font-medium hover:bg-accent"
				aria-expanded={open}
			>
				{open ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
				<span className="truncate">{schema}</span>
				<span className="ml-auto text-xs text-muted-foreground">{tables.length}</span>
			</button>
			{open ? (
				<ul className="ml-4 flex flex-col gap-0.5 border-l border-border pl-2">
					{tables.map((table) => {
						const active = selected?.schema === schema && selected?.table === table
						return (
							<li key={table}>
								<button
									type="button"
									onClick={() => onSelect(schema, table)}
									className={cn(
										'w-full truncate rounded-md px-2 py-1 text-left text-sm hover:bg-accent',
										active && 'bg-accent font-medium text-accent-foreground',
									)}
								>
									{table}
								</button>
							</li>
						)
					})}
				</ul>
			) : null}
		</li>
	)
}

function FilterBar({
	columns,
	filters,
	draft,
	setDraft,
	onApply,
	onRemove,
}: {
	columns: string[]
	filters: Filter[]
	draft: Filter
	setDraft: (f: Filter) => void
	onApply: () => void
	onRemove: (index: number) => void
}) {
	const needsValue = !UNARY_OPS.includes(draft.op)
	return (
		<div className="flex flex-col gap-2">
			<div className="flex flex-wrap items-end gap-2">
				<div className="flex flex-col gap-1">
					<Label className="text-xs text-muted-foreground">Column</Label>
					<input
						list="filter-columns"
						className="h-9 w-40 rounded-md border border-input bg-background px-2 text-sm"
						value={draft.column}
						onChange={(e) => setDraft({ ...draft, column: e.target.value })}
						placeholder="column"
					/>
					<datalist id="filter-columns">
						{columns.map((c) => (
							<option key={c} value={c} />
						))}
					</datalist>
				</div>
				<div className="flex flex-col gap-1">
					<Label className="text-xs text-muted-foreground">Operator</Label>
					<select
						className="h-9 rounded-md border border-input bg-background px-2 text-sm"
						value={draft.op}
						onChange={(e) => setDraft({ ...draft, op: e.target.value as FilterOp })}
					>
						{FILTER_OPS.map((op) => (
							<option key={op} value={op}>
								{op}
							</option>
						))}
					</select>
				</div>
				{needsValue ? (
					<div className="flex flex-col gap-1">
						<Label className="text-xs text-muted-foreground">Value</Label>
						<Input
							className="w-40"
							value={draft.value ?? ''}
							onChange={(e) => setDraft({ ...draft, value: e.target.value })}
							placeholder="value"
							onKeyDown={(e) => {
								if (e.key === 'Enter') onApply()
							}}
						/>
					</div>
				) : null}
				<Button variant="secondary" size="sm" onClick={onApply} disabled={!draft.column}>
					<Plus /> Add filter
				</Button>
			</div>

			{filters.length > 0 ? (
				<div className="flex flex-wrap gap-1.5">
					{filters.map((f, i) => (
						<Badge key={`${f.column}-${i}`} variant="secondary" className="font-mono">
							{f.column} ${f.op}
							{UNARY_OPS.includes(f.op) ? '' : `.${f.value ?? ''}`}
							<button
								type="button"
								onClick={() => onRemove(i)}
								aria-label={`Remove filter ${f.column}`}
								className="ml-1 hover:text-destructive"
							>
								<Trash2 className="size-3" />
							</button>
						</Badge>
					))}
				</div>
			) : null}
		</div>
	)
}

function RowsTable({
	columns,
	rows,
	loading,
}: {
	columns: string[]
	rows: Row[]
	loading: boolean
}) {
	if (loading && rows.length === 0) {
		return <p className="p-4 text-sm text-muted-foreground">Loading rows…</p>
	}
	if (rows.length === 0) {
		return <p className="p-4 text-sm text-muted-foreground">No rows.</p>
	}
	return (
		<div className="overflow-x-auto">
			<table className="w-full text-sm">
				<thead className="bg-muted/50 text-left text-xs text-muted-foreground">
					<tr>
						{columns.map((c) => (
							<th key={c} className="whitespace-nowrap px-3 py-2 font-medium">
								{c}
							</th>
						))}
					</tr>
				</thead>
				<tbody>
					{rows.map((row, i) => (
						<tr key={i} className="border-t border-border hover:bg-accent/40">
							{columns.map((c) => (
								<td
									key={c}
									className="max-w-xs truncate px-3 py-1.5 font-mono text-xs"
									title={renderCell(row[c])}
								>
									{renderCell(row[c])}
								</td>
							))}
						</tr>
					))}
				</tbody>
			</table>
		</div>
	)
}

function renderCell(value: unknown): string {
	if (value === null || value === undefined) return ''
	if (typeof value === 'object') return JSON.stringify(value)
	return String(value)
}
