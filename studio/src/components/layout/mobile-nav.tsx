import { Link } from '@tanstack/react-router'
import { NAV_ITEMS } from '@/components/layout/nav'
import { cn } from '@/lib/utils'

/** Bottom tab bar shown below the `md` breakpoint. */
export function MobileNav() {
	return (
		<nav
			aria-label="Primary"
			className="fixed inset-x-0 bottom-0 z-40 flex border-t border-border bg-card/95 backdrop-blur md:hidden"
		>
			{NAV_ITEMS.map((item) => {
				const Icon = item.icon
				return (
					<Link
						key={item.to}
						to={item.to}
						activeOptions={{ exact: item.to === '/' }}
						activeProps={{ className: 'text-primary' }}
						className={cn(
							'flex flex-1 flex-col items-center gap-1 py-2 text-xs font-medium text-muted-foreground',
							'transition-colors hover:text-foreground',
						)}
					>
						<Icon className="size-5" />
						<span>{item.label}</span>
					</Link>
				)
			})}
		</nav>
	)
}
