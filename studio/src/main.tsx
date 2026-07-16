import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from '@tanstack/react-router'
import { AppProviders } from '@/app/providers'
import { router } from '@/app/router'
import '@/styles.css'

const rootElement = document.getElementById('root')
if (!rootElement) {
	throw new Error('Studio: #root element not found')
}

createRoot(rootElement).render(
	<StrictMode>
		<AppProviders>
			<RouterProvider router={router} />
		</AppProviders>
	</StrictMode>,
)
