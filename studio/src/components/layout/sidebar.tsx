import { Link } from '@tanstack/react-router'
import { NAV_ITEMS } from '@/components/layout/nav'
import { cn } from '@/lib/utils'

/** Persistent sidebar navigation (visible from the `md` breakpoint up). */
export function Sidebar() {
	return (
		<nav aria-label="Primary" className="flex flex-col gap-1 p-3">
			{NAV_ITEMS.map((item) => {
				const Icon = item.icon
				return (
					<Link
						key={item.to}
						to={item.to}
						activeOptions={{ exact: item.to === '/' }}
						activeProps={{ className: 'bg-accent text-accent-foreground' }}
						className={cn(
							'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-muted-foreground',
							'transition-colors hover:bg-accent hover:text-accent-foreground',
						)}
					>
						<Icon className="size-4 shrink-0" />
						<span>{item.label}</span>
					</Link>
				)
			})}
		</nav>
	)
}
