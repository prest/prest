import { Database, LayoutDashboard, Send, Sparkles, type LucideIcon } from 'lucide-react'

export interface NavItem {
	label: string
	to: string
	icon: LucideIcon
	description: string
}

/** Primary navigation shared by the sidebar and the mobile nav. */
export const NAV_ITEMS: readonly NavItem[] = [
	{
		label: 'Overview',
		to: '/',
		icon: LayoutDashboard,
		description: 'Health, metadata and quick links',
	},
	{
		label: 'Data',
		to: '/data',
		icon: Database,
		description: 'Browse schemas, tables and rows',
	},
	{
		label: 'REST',
		to: '/rest',
		icon: Send,
		description: 'Build and run GET requests',
	},
	{
		label: 'MCP',
		to: '/mcp',
		icon: Sparkles,
		description: 'Inspect and invoke MCP tools',
	},
]
