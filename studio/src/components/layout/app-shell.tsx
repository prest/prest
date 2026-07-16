import * as React from 'react'
import { Outlet } from '@tanstack/react-router'
import { KeyRound, ShieldCheck } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Sidebar } from '@/components/layout/sidebar'
import { MobileNav } from '@/components/layout/mobile-nav'
import { ThemeToggle } from '@/components/layout/theme-toggle'
import { AuthDialog } from '@/components/layout/auth-dialog'
import { useAuth } from '@/app/providers'
import { maskToken } from '@/lib/auth/token'

/** Top-level chrome: header, responsive navigation and routed content. */
export function AppShell() {
	const [authOpen, setAuthOpen] = React.useState(false)
	const { token, hasToken } = useAuth()

	return (
		<div className="flex min-h-full flex-col">
			<header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-border bg-card/95 px-4 backdrop-blur">
				<div className="flex items-center gap-2 font-semibold">
					<span className="grid size-7 place-items-center rounded-md bg-primary text-primary-foreground">
						p
					</span>
					<span>
						pREST <span className="text-muted-foreground">Studio</span>
					</span>
				</div>

				<div className="ml-auto flex items-center gap-2">
					<Button
						variant="outline"
						size="sm"
						onClick={() => setAuthOpen(true)}
						aria-label="Configure bearer token"
					>
						{hasToken ? <ShieldCheck className="text-success" /> : <KeyRound />}
						<span className="hidden sm:inline">
							{hasToken ? (
								<span className="font-mono text-xs">{maskToken(token)}</span>
							) : (
								'Set token'
							)}
						</span>
						{hasToken ? (
							<Badge variant="success" className="hidden md:inline-flex">
								auth
							</Badge>
						) : null}
					</Button>
					<ThemeToggle />
				</div>
			</header>

			<div className="flex flex-1">
				<aside className="hidden w-60 shrink-0 border-r border-border md:block">
					<div className="sticky top-14">
						<Sidebar />
					</div>
				</aside>

				<main className="min-w-0 flex-1 px-4 pb-20 pt-6 md:px-8 md:pb-8">
					<div className="mx-auto w-full max-w-6xl">
						<Outlet />
					</div>
				</main>
			</div>

			<MobileNav />
			<AuthDialog open={authOpen} onOpenChange={setAuthOpen} />
		</div>
	)
}
