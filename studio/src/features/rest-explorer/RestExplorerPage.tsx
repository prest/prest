import * as React from 'react'
import { Play, Plus, Trash2 } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/copy-button'
import { usePrestClient } from '@/app/providers'
import { buildCurl } from '@/lib/api/curl'
import { toErrorMessage } from '@/lib/errors'

interface Param {
	id: string
	key: string
	value: string
	enabled: boolean
}

interface ResponseState {
	status: number
	ok: boolean
	durationMs: number
	body: string
	contentType: string
}

function newParam(partial?: Partial<Omit<Param, 'id'>>): Param {
	return {
		id: crypto.randomUUID(),
		key: '',
		value: '',
		enabled: true,
		...partial,
	}
}

function buildQuery(params: Param[]): string {
	const sp = new URLSearchParams()
	for (const p of params) {
		if (p.enabled && p.key) sp.append(p.key, p.value)
	}
	return sp.toString()
}

function prettify(text: string, contentType: string): string {
	if (!contentType.includes('json')) return text
	try {
		return JSON.stringify(JSON.parse(text), null, 2)
	} catch {
		return text
	}
}

export function RestExplorerPage() {
	const client = usePrestClient()
	const [path, setPath] = React.useState('/databases')
	const [params, setParams] = React.useState<Param[]>(() => [newParam()])
	const [response, setResponse] = React.useState<ResponseState | null>(null)
	const [error, setError] = React.useState<string | null>(null)
	const [loading, setLoading] = React.useState(false)

	const query = buildQuery(params)
	const relativeUrl = client.resolve(path, query)
	const curl = buildCurl({ url: relativeUrl, origin: window.location.origin })

	const send = async () => {
		setLoading(true)
		setError(null)
		try {
			const res = await client.requestRaw(path, { method: 'GET', query })
			setResponse({
				status: res.status,
				ok: res.ok,
				durationMs: res.durationMs,
				contentType: res.headers.get('content-type') ?? '',
				body: res.text,
			})
		} catch (err) {
			setResponse(null)
			setError(toErrorMessage(err))
		} finally {
			setLoading(false)
		}
	}

	const updateParam = (id: string, patch: Partial<Param>) => {
		setParams((prev) => prev.map((p) => (p.id === id ? { ...p, ...patch } : p)))
	}

	return (
		<div className="flex flex-col gap-4">
			<div>
				<h1 className="text-2xl font-semibold">REST Explorer</h1>
				<p className="text-sm text-muted-foreground">
					Build and run <span className="font-mono">GET</span> requests against the pREST API.
				</p>
			</div>

			<Card>
				<CardContent className="flex flex-col gap-4 p-4">
					<div className="flex flex-wrap items-end gap-2">
						<Badge variant="outline" className="h-9 rounded-md px-3 font-mono">
							GET
						</Badge>
						<div className="flex min-w-[16rem] flex-1 flex-col gap-1">
							<Label htmlFor="rest-path">Path</Label>
							<Input
								id="rest-path"
								value={path}
								onChange={(e) => setPath(e.target.value)}
								placeholder="/{database}/{schema}/{table}"
								className="font-mono"
								onKeyDown={(e) => {
									if (e.key === 'Enter' && !loading) void send()
								}}
							/>
						</div>
						<Button onClick={() => void send()} disabled={loading}>
							<Play /> {loading ? 'Sending…' : 'Send'}
						</Button>
					</div>

					<div className="flex flex-col gap-2">
						<Label className="text-xs text-muted-foreground">Query parameters</Label>
						{params.map((p) => (
							<div key={p.id} className="flex items-center gap-2">
								<input
									type="checkbox"
									className="size-4 accent-primary"
									checked={p.enabled}
									onChange={(e) => updateParam(p.id, { enabled: e.target.checked })}
									aria-label="Enable parameter"
								/>
								<Input
									className="flex-1 font-mono"
									placeholder="key"
									value={p.key}
									onChange={(e) => updateParam(p.id, { key: e.target.value })}
								/>
								<Input
									className="flex-1 font-mono"
									placeholder="value (e.g. $eq.1)"
									value={p.value}
									onChange={(e) => updateParam(p.id, { value: e.target.value })}
								/>
								<Button
									variant="ghost"
									size="icon"
									onClick={() => setParams((prev) => prev.filter((x) => x.id !== p.id))}
									aria-label="Remove parameter"
								>
									<Trash2 />
								</Button>
							</div>
						))}
						<div>
							<Button
								variant="secondary"
								size="sm"
								onClick={() => setParams((prev) => [...prev, newParam()])}
							>
								<Plus /> Add parameter
							</Button>
						</div>
					</div>

					<div className="flex flex-wrap items-center gap-2">
						<code className="flex-1 overflow-x-auto rounded-md bg-muted px-3 py-2 font-mono text-xs">
							{relativeUrl}
						</code>
						<CopyButton value={relativeUrl} label="Copy URL" />
						<CopyButton value={curl} label="Copy curl" />
					</div>
				</CardContent>
			</Card>

			<Card>
				<CardHeader className="flex-row items-center justify-between gap-2 space-y-0">
					<CardTitle className="text-sm">Response</CardTitle>
					{response ? (
						<div className="flex items-center gap-2 text-xs">
							<Badge variant={response.ok ? 'success' : 'destructive'}>
								HTTP {response.status}
							</Badge>
							<span className="text-muted-foreground">{response.durationMs}ms</span>
						</div>
					) : null}
				</CardHeader>
				<CardContent>
					{error ? (
						<p className="text-sm text-destructive">{error}</p>
					) : response ? (
						<pre className="max-h-[50vh] overflow-auto rounded-md bg-muted p-3 font-mono text-xs">
							{prettify(response.body, response.contentType) || '(empty response body)'}
						</pre>
					) : (
						<p className="text-sm text-muted-foreground">Send a request to see the response.</p>
					)}
				</CardContent>
			</Card>
		</div>
	)
}
