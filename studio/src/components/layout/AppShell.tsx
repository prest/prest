import * as React from 'react'
import { Link, useRouterState } from '@tanstack/react-router'
import { Database, Home, Menu, Network, Terminal, KeyRound, Moon, Sun, Monitor } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogTitle,
	DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuth, useTheme } from '@/app/providers'
import { maskToken } from '@/lib/auth/token'
import { cn } from '@/lib/utils'

const nav = [
	{ to: '/', label: 'Overview', icon: Home },
	{ to: '/data', label: 'Data Explorer', icon: Database },
	{ to: '/rest', label: 'REST Explorer', icon: Terminal },
	{ to: '/mcp', label: 'MCP Explorer', icon: Network },
] as const

function AuthDialog() {
	const auth = useAuth()
	const [open, setOpen] = React.useState(false)
	const [value, setValue] = React.useState('')
	const [remember, setRemember] = React.useState(false)

	React.useEffect(() => {
		if (open) {
			setValue('')
			setRemember(auth.remembered)
		}
	}, [open, auth.remembered])

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger asChild>
				<Button variant="outline" size="sm" aria-label="Session token">
					<KeyRound className="h-4 w-4" />
					{auth.token ? 'Token' : 'Set token'}
				</Button>
			</DialogTrigger>
			<DialogContent>
				<DialogTitle>Session token</DialogTitle>
				<DialogDescription>
					Bearer token is kept in memory by default. Choose “remember for this tab” to use session
					storage only — never local storage.
				</DialogDescription>
				{auth.token ? (
					<p className="mt-3 font-mono text-xs text-[var(--muted-fg)]">
						Active: {maskToken(auth.token)}
					</p>
				) : null}
				<div className="mt-4 space-y-3">
					<div className="space-y-1">
						<Label htmlFor="token">Bearer token</Label>
						<Input
							id="token"
							type="password"
							autoComplete="off"
							value={value}
							onChange={(e) => setValue(e.target.value)}
							placeholder="Paste JWT or API token"
						/>
					</div>
					<label className="flex items-center gap-2 text-sm">
						<input
							type="checkbox"
							checked={remember}
							onChange={(e) => setRemember(e.target.checked)}
						/>
						Remember for this tab
					</label>
					<div className="flex gap-2">
						<Button
							onClick={() => {
								auth.setToken(value, remember)
								setOpen(false)
							}}
						>
							Save
						</Button>
						<Button
							variant="secondary"
							onClick={() => {
								auth.clearToken()
								setOpen(false)
							}}
						>
							Remove token
						</Button>
					</div>
				</div>
			</DialogContent>
		</Dialog>
	)
}

function ThemeToggle() {
	const { theme, setTheme } = useTheme()
	const cycle = () => {
		setTheme(theme === 'system' ? 'light' : theme === 'light' ? 'dark' : 'system')
	}
	const Icon = theme === 'dark' ? Moon : theme === 'light' ? Sun : Monitor
	return (
		<Button variant="ghost" size="icon" onClick={cycle} aria-label={`Theme: ${theme}`}>
			<Icon className="h-4 w-4" />
		</Button>
	)
}

export function AppShell({ children }: { children: React.ReactNode }) {
	const [mobileOpen, setMobileOpen] = React.useState(false)
	const pathname = useRouterState({ select: (s) => s.location.pathname })

	return (
		<div className="flex h-full min-h-0">
			<aside className="hidden w-56 shrink-0 flex-col bg-[var(--sidebar)] text-[var(--sidebar-fg)] md:flex">
				<div className="border-b border-white/10 px-4 py-4">
					<div className="text-sm font-semibold tracking-wide">pREST Studio</div>
					<div className="text-xs text-[var(--sidebar-muted)]">REST &amp; MCP control plane</div>
				</div>
				<nav className="flex flex-1 flex-col gap-1 p-2" aria-label="Primary">
					{nav.map((item) => {
						const active =
							item.to === '/' ? pathname === '/' || pathname === '' : pathname.startsWith(item.to)
						const Icon = item.icon
						return (
							<Link
								key={item.to}
								to={item.to}
								className={cn(
									'flex items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-white/10',
									active && 'bg-white/15 font-medium',
								)}
							>
								<Icon className="h-4 w-4" />
								{item.label}
							</Link>
						)
					})}
				</nav>
			</aside>

			<div className="flex min-w-0 flex-1 flex-col">
				<header className="flex items-center justify-between border-b border-[var(--border)] bg-[var(--card)] px-3 py-2">
					<div className="flex items-center gap-2 md:hidden">
						<Button
							variant="ghost"
							size="icon"
							aria-label="Open menu"
							onClick={() => setMobileOpen((v) => !v)}
						>
							<Menu className="h-4 w-4" />
						</Button>
						<span className="text-sm font-semibold">pREST Studio</span>
					</div>
					<div className="ml-auto flex items-center gap-2">
						<ThemeToggle />
						<AuthDialog />
					</div>
				</header>
				{mobileOpen ? (
					<nav
						className="flex flex-col gap-1 border-b border-[var(--border)] bg-[var(--card)] p-2 md:hidden"
						aria-label="Mobile"
					>
						{nav.map((item) => (
							<Link
								key={item.to}
								to={item.to}
								className="rounded-md px-3 py-2 text-sm hover:bg-[var(--muted)]"
								onClick={() => setMobileOpen(false)}
							>
								{item.label}
							</Link>
						))}
					</nav>
				) : null}
				<main className="min-h-0 flex-1 overflow-auto p-4">{children}</main>
			</div>
		</div>
	)
}
