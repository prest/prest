import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PrestClient } from '@/lib/api/client'
import { PrestClientProvider } from '@/app/providers'
import { RestExplorerPage } from './RestExplorerPage'

// Wiring only: the implementations live in W1-studio-lib-api.
const formatJsonText = vi.fn((text: string) => `FORMATTED(${text})`)
const absoluteUrl = vi.fn((rel: string, origin: string) => `${origin}${rel}`)
vi.mock('@/lib/api/json', () => ({
	formatJsonText: (text: string) => formatJsonText(text),
}))
vi.mock('@/lib/api/url', async (orig) => ({
	...(await orig<typeof import('@/lib/api/url')>()),
	absoluteUrl: (rel: string, origin: string) => absoluteUrl(rel, origin),
}))

function stubClient(contentType: string, text: string): PrestClient {
	return {
		resolve: (path: string, query?: string) => (query ? `${path}?${query}` : path),
		requestRaw: vi.fn().mockResolvedValue({
			status: 200,
			ok: true,
			durationMs: 3,
			headers: new Headers({ 'content-type': contentType }),
			text,
		}),
	} as unknown as PrestClient
}

function renderPage(client: PrestClient) {
	render(
		<PrestClientProvider client={client}>
			<RestExplorerPage />
		</PrestClientProvider>,
	)
}

describe('RestExplorerPage', { concurrent: false }, () => {
	beforeEach(() => {
		formatJsonText.mockClear()
		absoluteUrl.mockClear()
	})

	it('renders formatJsonText output for JSON responses', async () => {
		renderPage(stubClient('application/json', '[{"id":9007199254740993}]'))
		await userEvent.click(screen.getByRole('button', { name: /send/i }))
		expect(await screen.findByText('FORMATTED([{"id":9007199254740993}])')).toBeInTheDocument()
		expect(formatJsonText).toHaveBeenCalledWith('[{"id":9007199254740993}]')
	})

	it('shows non-JSON bodies as-is', async () => {
		renderPage(stubClient('text/plain', 'hello'))
		await userEvent.click(screen.getByRole('button', { name: /send/i }))
		expect(await screen.findByText('hello')).toBeInTheDocument()
		expect(formatJsonText).not.toHaveBeenCalled()
	})

	it('copies an absolute URL but displays the relative one', async () => {
		const writeText = vi.fn().mockResolvedValue(undefined)
		Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
		renderPage(stubClient('application/json', '[]'))
		expect(screen.getByText('/databases')).toBeInTheDocument()
		await userEvent.click(screen.getByRole('button', { name: 'Copy URL' }))
		expect(absoluteUrl).toHaveBeenCalledWith('/databases', window.location.origin)
		expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/databases`)
	})
})
