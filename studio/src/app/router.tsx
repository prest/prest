import {
	createRootRoute,
	createRoute,
	createRouter,
	lazyRouteComponent,
} from '@tanstack/react-router'
import { AppShell } from '@/components/layout/app-shell'
import { OverviewPage } from '@/features/overview/OverviewPage'
import { dataSearchSchema } from '@/features/catalog/search'

const rootRoute = createRootRoute({
	component: AppShell,
})

const overviewRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/',
	component: OverviewPage,
})

const dataRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/data',
	validateSearch: dataSearchSchema,
	component: lazyRouteComponent(
		() => import('@/features/catalog/DataExplorerPage'),
		'DataExplorerPage',
	),
})

const restRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/rest',
	component: lazyRouteComponent(
		() => import('@/features/rest-explorer/RestExplorerPage'),
		'RestExplorerPage',
	),
})

const mcpRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/mcp',
	component: lazyRouteComponent(() => import('@/features/mcp/McpExplorerPage'), 'McpExplorerPage'),
})

const routeTree = rootRoute.addChildren([overviewRoute, dataRoute, restRoute, mcpRoute])

export function createAppRouter() {
	return createRouter({
		routeTree,
		basepath: '/_studio',
		defaultPreload: 'intent',
		scrollRestoration: true,
	})
}

export const router = createAppRouter()

declare module '@tanstack/react-router' {
	interface Register {
		router: typeof router
	}
}
