import * as React from 'react'
import { Bot, Play, Plug, Search, Wand2 } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/copy-button'
import { usePrestClient, useAuth } from '@/app/providers'
import {
	McpClient,
	McpError,
	contentToText,
	type InitializeResult,
	type McpTool,
} from '@/lib/mcp/client'
import { cn } from '@/lib/utils'

interface HistoryEntry {
	id: number
	tool: string
	args: Record<string, unknown>
	ok: boolean
	output: string
	at: string
}

type SchemaProp = { type?: string; description?: string }

function schemaProps(tool: McpTool | null): [string, SchemaProp][] {
	const schema = tool?.inputSchema
	const props = schema && typeof schema === 'object' ? (schema.properties as unknown) : undefined
	if (!props || typeof props !== 'object') return []
	return Object.entries(props as Record<string, SchemaProp>)
}

function requiredFields(tool: McpTool | null): Set<string> {
	const req = tool?.inputSchema?.required
	return new Set(Array.isArray(req) ? (req as string[]) : [])
}

function coerce(value: string, type?: string): unknown {
	if (type === 'number' || type === 'integer') {
		const n = Number(value)
		return Number.isNaN(n) ? value : n
	}
	if (type === 'boolean') return value === 'true'
	return value
}

export function McpExplorerPage() {
	const client = usePrestClient()
	const { hasToken } = useAuth()
	const mcp = React.useMemo(() => new McpClient(client), [client])

	const [init, setInit] = React.useState<InitializeResult | null>(null)
	const [connecting, setConnecting] = React.useState(false)
	const [connectError, setConnectError] = React.useState<string | null>(null)

	const [tools, setTools] = React.useState<McpTool[]>([])
	const [filter, setFilter] = React.useState('')
	const [selected, setSelected] = React.useState<McpTool | null>(null)

	const [rawMode, setRawMode] = React.useState(false)
	const [rawArgs, setRawArgs] = React.useState('{}')
	const [formArgs, setFormArgs] = React.useState<Record<string, string>>({})
	const [invoking, setInvoking] = React.useState(false)
	const [result, setResult] = React.useState<{ ok: boolean; output: string } | null>(null)
	const [history, setHistory] = React.useState<HistoryEntry[]>([])

	const connect = async () => {
		setConnecting(true)
		setConnectError(null)
		setTools([])
		setSelected(null)
		setResult(null)
		try {
			const info = await mcp.initialize()
			setInit(info)
			const list = await mcp.listTools()
			setTools(list.tools)
		} catch (err) {
			setConnectError(err instanceof McpError ? err.message : String(err))
			setInit(null)
			setTools([])
			setSelected(null)
			setResult(null)
		} finally {
			setConnecting(false)
		}
	}

	const selectTool = (tool: McpTool) => {
		setSelected(tool)
		setResult(null)
		setRawArgs('{}')
		setFormArgs({})
		setRawMode(false)
	}

	const buildArgs = (): Record<string, unknown> => {
		if (rawMode) {
			const parsed = JSON.parse(rawArgs || '{}')
			if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
				throw new Error('Arguments must be a JSON object.')
			}
			return parsed as Record<string, unknown>
		}
		const required = requiredFields(selected)
		const args: Record<string, unknown> = {}
		for (const [name, prop] of schemaProps(selected)) {
			const raw = formArgs[name]
			if (raw !== undefined && raw !== '') args[name] = coerce(raw, prop.type)
		}
		const missing = [...required].filter((name) => !(name in args))
		if (missing.length > 0) {
			throw new Error(`Missing required fields: ${missing.join(', ')}`)
		}
		return args
	}

	const invoke = async () => {
		if (!selected) return
		setInvoking(true)
		setResult(null)
		let args: Record<string, unknown> = {}
		try {
			args = buildArgs()
		} catch (err) {
			setResult({ ok: false, output: err instanceof Error ? err.message : String(err) })
			setInvoking(false)
			return
		}
		try {
			const res = await mcp.callTool(selected.name, args)
			const output = contentToText(res)
			const ok = !res.isError
			setResult({ ok, output })
			pushHistory(selected.name, args, ok, output)
		} catch (err) {
			const output = err instanceof McpError ? err.message : String(err)
			setResult({ ok: false, output })
			pushHistory(selected.name, args, false, output)
		} finally {
			setInvoking(false)
		}
	}

	const pushHistory = (
		tool: string,
		args: Record<string, unknown>,
		ok: boolean,
		output: string,
	) => {
		setHistory((prev) =>
			[
				{ id: Date.now(), tool, args, ok, output, at: new Date().toLocaleTimeString() },
				...prev,
			].slice(0, 20),
		)
	}

	const filtered = tools.filter(
		(t) =>
			t.name.toLowerCase().includes(filter.toLowerCase()) ||
			(t.description ?? '').toLowerCase().includes(filter.toLowerCase()),
	)

	return (
		<div className="flex flex-col gap-4">
			<div className="flex flex-wrap items-center justify-between gap-2">
				<div>
					<h1 className="text-2xl font-semibold">MCP Explorer</h1>
					<p className="text-sm text-muted-foreground">
						Connect to the <span className="font-mono">/_mcp</span> endpoint and invoke tools.
					</p>
				</div>
				<div className="flex items-center gap-2">
					{init ? (
						<Badge variant="success">
							<Plug className="size-3" /> connected
						</Badge>
					) : null}
					<Button onClick={() => void connect()} disabled={connecting}>
						<Plug /> {connecting ? 'Connecting…' : init ? 'Reconnect' : 'Connect'}
					</Button>
				</div>
			</div>

			{connectError ? (
				<Card>
					<CardContent className="p-4 text-sm text-destructive">{connectError}</CardContent>
				</Card>
			) : null}

			{init ? (
				<Card>
					<CardContent className="flex flex-wrap gap-x-6 gap-y-1 p-4 text-sm">
						<span>
							<span className="text-muted-foreground">Server: </span>
							<span className="font-medium">{init.serverInfo?.name ?? 'unknown'}</span>
						</span>
						<span>
							<span className="text-muted-foreground">Version: </span>
							{init.serverInfo?.version ?? '—'}
						</span>
						<span>
							<span className="text-muted-foreground">Protocol: </span>
							{init.protocolVersion ?? '—'}
						</span>
						<span>
							<span className="text-muted-foreground">Tools: </span>
							{tools.length}
						</span>
					</CardContent>
				</Card>
			) : null}

			<div className="grid gap-4 lg:grid-cols-[18rem_1fr]">
				<Card className="h-fit">
					<CardHeader className="gap-2">
						<CardTitle className="text-sm">Tools</CardTitle>
						<div className="relative">
							<Search className="absolute left-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
							<Input
								className="pl-8"
								placeholder="Search tools…"
								value={filter}
								onChange={(e) => setFilter(e.target.value)}
								disabled={tools.length === 0}
							/>
						</div>
					</CardHeader>
					<CardContent className="max-h-[60vh] overflow-auto">
						{tools.length === 0 ? (
							<p className="text-xs text-muted-foreground">
								{init ? 'No tools reported.' : 'Connect to list tools.'}
							</p>
						) : (
							<ul className="flex flex-col gap-0.5">
								{filtered.map((tool) => (
									<li key={tool.name}>
										<button
											type="button"
											onClick={() => selectTool(tool)}
											className={cn(
												'w-full rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent',
												selected?.name === tool.name && 'bg-accent font-medium',
											)}
										>
											<span className="block truncate font-mono text-xs">{tool.name}</span>
											{tool.description ? (
												<span className="block truncate text-xs text-muted-foreground">
													{tool.description}
												</span>
											) : null}
										</button>
									</li>
								))}
							</ul>
						)}
					</CardContent>
				</Card>

				<div className="flex min-w-0 flex-col gap-4">
					{selected ? (
						<Card>
							<CardHeader className="flex-row items-center justify-between gap-2 space-y-0">
								<div className="min-w-0">
									<CardTitle className="truncate font-mono text-sm">{selected.name}</CardTitle>
									{selected.description ? (
										<CardDescription>{selected.description}</CardDescription>
									) : null}
								</div>
								<Button variant="ghost" size="sm" onClick={() => setRawMode((v) => !v)}>
									{rawMode ? 'Form' : 'Raw JSON'}
								</Button>
							</CardHeader>
							<CardContent className="flex flex-col gap-3">
								{rawMode ? (
									<textarea
										className="min-h-32 w-full rounded-md border border-input bg-background p-3 font-mono text-xs"
										value={rawArgs}
										onChange={(e) => setRawArgs(e.target.value)}
										spellCheck={false}
									/>
								) : schemaProps(selected).length === 0 ? (
									<p className="text-xs text-muted-foreground">This tool takes no arguments.</p>
								) : (
									<div className="flex flex-col gap-2">
										{schemaProps(selected).map(([name, prop]) => {
											const required = requiredFields(selected).has(name)
											return (
												<div key={name} className="flex flex-col gap-1">
													<Label htmlFor={`arg-${name}`} className="text-xs">
														<span className="font-mono">{name}</span>
														{prop.type ? (
															<span className="text-muted-foreground"> : {prop.type}</span>
														) : null}
														{required ? <span className="text-destructive"> *</span> : null}
													</Label>
													<Input
														id={`arg-${name}`}
														value={formArgs[name] ?? ''}
														onChange={(e) =>
															setFormArgs((prev) => ({ ...prev, [name]: e.target.value }))
														}
														placeholder={prop.description ?? prop.type ?? ''}
													/>
												</div>
											)
										})}
									</div>
								)}

								<div>
									<Button onClick={() => void invoke()} disabled={invoking}>
										<Play /> {invoking ? 'Invoking…' : 'Invoke tool'}
									</Button>
								</div>

								{result ? (
									<div className="flex flex-col gap-1">
										<div className="flex items-center gap-2">
											<Badge variant={result.ok ? 'success' : 'destructive'}>
												{result.ok ? 'success' : 'error'}
											</Badge>
											<CopyButton value={result.output} label="Copy" />
										</div>
										<pre className="max-h-72 overflow-auto rounded-md bg-muted p-3 font-mono text-xs">
											{result.output || '(empty result)'}
										</pre>
									</div>
								) : null}
							</CardContent>
						</Card>
					) : (
						<Card>
							<CardContent className="p-8 text-center text-sm text-muted-foreground">
								{init ? 'Select a tool to invoke it.' : 'Connect to the MCP endpoint to begin.'}
							</CardContent>
						</Card>
					)}

					<ConnectAiPanel hasToken={hasToken} />

					{history.length > 0 ? (
						<Card>
							<CardHeader>
								<CardTitle className="text-sm">History</CardTitle>
							</CardHeader>
							<CardContent className="flex flex-col gap-2">
								{history.map((h) => (
									<div key={h.id} className="rounded-md border border-border p-2 text-xs">
										<div className="flex items-center gap-2">
											<Badge variant={h.ok ? 'success' : 'destructive'}>
												{h.ok ? 'ok' : 'err'}
											</Badge>
											<span className="font-mono">{h.tool}</span>
											<span className="ml-auto text-muted-foreground">{h.at}</span>
										</div>
										<pre className="mt-1 overflow-x-auto font-mono text-muted-foreground">
											{JSON.stringify(h.args)}
										</pre>
									</div>
								))}
							</CardContent>
						</Card>
					) : null}
				</div>
			</div>
		</div>
	)
}

function ConnectAiPanel({ hasToken }: { hasToken: boolean }) {
	const origin = typeof window !== 'undefined' ? window.location.origin : ''
	const config = JSON.stringify(
		{
			mcpServers: {
				prest: {
					url: `${origin}/_mcp`,
					headers: { Authorization: 'Bearer <YOUR_TOKEN>' },
				},
			},
		},
		null,
		2,
	)

	return (
		<Card>
			<CardHeader>
				<CardTitle className="flex items-center gap-2 text-sm">
					<Bot className="size-4" /> Connect an AI client
				</CardTitle>
				<CardDescription>
					Point an MCP-capable client (Claude, Cursor, …) at this endpoint.
				</CardDescription>
			</CardHeader>
			<CardContent className="flex flex-col gap-2">
				<div className="flex items-center gap-2">
					<Wand2 className="size-4 text-muted-foreground" />
					<code className="flex-1 overflow-x-auto rounded-md bg-muted px-3 py-2 font-mono text-xs">
						{origin}/_mcp
					</code>
					<CopyButton value={`${origin}/_mcp`} label="Copy URL" />
				</div>
				<pre className="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs">{config}</pre>
				<div className="flex items-center justify-between">
					<p className="text-xs text-muted-foreground">
						{hasToken
							? 'Replace <YOUR_TOKEN> with your bearer token (kept out of this snippet for safety).'
							: 'Add a bearer token above if the endpoint requires authentication.'}
					</p>
					<CopyButton value={config} label="Copy config" />
				</div>
			</CardContent>
		</Card>
	)
}
