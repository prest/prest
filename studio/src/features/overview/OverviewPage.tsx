import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Activity, ArrowRight, CheckCircle2, Database, Send, Sparkles, XCircle } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { usePrestClient, useAuth } from '@/app/providers'
import {
	getHealth,
	getMeta,
	getReady,
	listDatabases,
	listSchemas,
	listTables,
	probeMcp,
} from '@/lib/api/prest'
import type { PrestClient } from '@/lib/api/client'

function useProbe(name: string, fn: (c: PrestClient) => Promise<{ ok: boolean; status: number }>) {
	const client = usePrestClient()
	return useQuery({
		queryKey: ['probe', name],
		queryFn: () => fn(client),
	})
}

function StatusRow({
	label,
	ok,
	detail,
	isError,
}: {
	label: string
	ok?: boolean
	detail?: string
	isError?: boolean
}) {
	return (
		<div className="flex items-center justify-between gap-2 py-1.5">
			<span className="text-sm text-muted-foreground">{label}</span>
			<span className="flex items-center gap-1.5 text-sm font-medium">
				{isError ? (
					<>
						<XCircle className="size-4 text-destructive" /> {detail ?? 'Error'}
					</>
				) : ok === undefined ? (
					<span className="text-muted-foreground">—</span>
				) : ok ? (
					<>
						<CheckCircle2 className="size-4 text-success" /> {detail ?? 'OK'}
					</>
				) : (
					<>
						<XCircle className="size-4 text-destructive" /> {detail ?? 'Down'}
					</>
				)}
			</span>
		</div>
	)
}

function CountCard({
	icon: Icon,
	label,
	count,
	disabled,
	error,
	isError,
}: {
	icon: typeof Database
	label: string
	count?: number
	disabled?: boolean
	error?: string
	isError?: boolean
}) {
	return (
		<Card>
			<CardContent className="flex items-center gap-3 p-4">
				<div className="grid size-10 place-items-center rounded-md bg-accent text-accent-foreground">
					<Icon className="size-5" />
				</div>
				<div className="min-w-0">
					<div className="text-2xl font-semibold tabular-nums">
						{disabled ? '—' : isError || error ? '!' : (count ?? '…')}
					</div>
					<div className="truncate text-xs text-muted-foreground">
						{disabled
							? `${label} (restricted)`
							: isError
								? `${label} (failed)`
								: error
									? `${label} (${error})`
									: label}
					</div>
				</div>
			</CardContent>
		</Card>
	)
}

const QUICK_LINKS = [
	{ to: '/data', label: 'Data Explorer', icon: Database, desc: 'Browse tables & rows' },
	{ to: '/rest', label: 'REST Explorer', icon: Send, desc: 'Build GET requests' },
	{ to: '/mcp', label: 'MCP Explorer', icon: Sparkles, desc: 'Inspect MCP tools' },
] as const

export function OverviewPage() {
	const client = usePrestClient()
	const { hasToken } = useAuth()

	const meta = useQuery({ queryKey: ['meta'], queryFn: () => getMeta(client) })
	const health = useProbe('health', getHealth)
	const ready = useProbe('ready', getReady)
	const mcp = useProbe('mcp', probeMcp)

	const databases = useQuery({ queryKey: ['databases'], queryFn: () => listDatabases(client) })
	const schemas = useQuery({ queryKey: ['schemas'], queryFn: () => listSchemas(client) })
	const tables = useQuery({ queryKey: ['tables'], queryFn: () => listTables(client) })

	return (
		<div className="flex flex-col gap-6">
			<div>
				<h1 className="text-2xl font-semibold">Overview</h1>
				<p className="text-sm text-muted-foreground">
					Server status, catalog counts and quick links.
				</p>
			</div>

			<div className="grid gap-4 md:grid-cols-3">
				<CountCard
					icon={Database}
					label="Databases"
					count={databases.data?.items?.length}
					disabled={databases.data?.disabled}
					error={databases.data?.error}
					isError={databases.isError}
				/>
				<CountCard
					icon={Database}
					label="Schemas"
					count={schemas.data?.items?.length}
					disabled={schemas.data?.disabled}
					error={schemas.data?.error}
					isError={schemas.isError}
				/>
				<CountCard
					icon={Database}
					label="Tables"
					count={tables.data?.items?.length}
					disabled={tables.data?.disabled}
					error={tables.data?.error}
					isError={tables.isError}
				/>
			</div>

			<div className="grid gap-4 md:grid-cols-2">
				<Card>
					<CardHeader>
						<CardTitle className="flex items-center gap-2">
							<Activity className="size-4" /> Server status
						</CardTitle>
					</CardHeader>
					<CardContent>
						<StatusRow
							label="Health (/_health)"
							ok={health.isSuccess ? health.data?.ok : undefined}
							detail={
								health.isError
									? 'Request failed'
									: health.data
										? `HTTP ${health.data.status}`
										: undefined
							}
							isError={health.isError}
						/>
						<StatusRow
							label="Ready (/_ready)"
							ok={ready.isSuccess ? ready.data?.ok : undefined}
							detail={
								ready.isError
									? 'Request failed'
									: ready.data
										? `HTTP ${ready.data.status}`
										: undefined
							}
							isError={ready.isError}
						/>
						<StatusRow
							label="MCP (/_mcp)"
							ok={mcp.isSuccess ? mcp.data?.ok : undefined}
							detail={
								mcp.isError ? 'Request failed' : mcp.data ? `HTTP ${mcp.data.status}` : undefined
							}
							isError={mcp.isError}
						/>
						<StatusRow label="Bearer token" ok={hasToken} detail={hasToken ? 'Set' : 'Not set'} />
					</CardContent>
				</Card>

				<Card>
					<CardHeader>
						<CardTitle>Server metadata</CardTitle>
						<CardDescription>Reported by /_studio/api/meta</CardDescription>
					</CardHeader>
					<CardContent className="flex flex-col gap-1.5 text-sm">
						{meta.isError ? (
							<span className="text-destructive">Failed to load metadata.</span>
						) : (
							<>
								<MetaRow label="Version" value={meta.data?.version} />
								<MetaRow label="Commit" value={meta.data?.commit} mono />
								<MetaRow label="Build date" value={meta.data?.buildDate} />
								<MetaRow label="API base" value={meta.data?.apiBasePath} mono />
								<MetaRow label="MCP endpoint" value={meta.data?.mcpEndpoint} mono />
							</>
						)}
					</CardContent>
				</Card>
			</div>

			<div>
				<h2 className="mb-3 text-sm font-medium text-muted-foreground">Quick links</h2>
				<div className="grid gap-4 sm:grid-cols-3">
					{QUICK_LINKS.map((q) => {
						const Icon = q.icon
						return (
							<Link key={q.to} to={q.to} className="group">
								<Card className="h-full transition-colors hover:border-primary">
									<CardContent className="flex items-center gap-3 p-4">
										<div className="grid size-10 place-items-center rounded-md bg-primary/10 text-primary">
											<Icon className="size-5" />
										</div>
										<div className="min-w-0 flex-1">
											<div className="flex items-center gap-1 font-medium">
												{q.label}
												<ArrowRight className="size-4 opacity-0 transition-opacity group-hover:opacity-100" />
											</div>
											<div className="text-xs text-muted-foreground">{q.desc}</div>
										</div>
									</CardContent>
								</Card>
							</Link>
						)
					})}
				</div>
			</div>
		</div>
	)
}

function MetaRow({ label, value, mono }: { label: string; value?: string; mono?: boolean }) {
	return (
		<div className="flex items-center justify-between gap-2">
			<span className="text-muted-foreground">{label}</span>
			<span className={mono ? 'font-mono text-xs' : ''}>{value || '—'}</span>
		</div>
	)
}
