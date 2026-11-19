import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClientProvider, useQuery } from '@tanstack/react-query'
import { TokenStore } from '@/lib/auth/token'
import { AuthProvider, createQueryClient, useAuth } from './providers'

/** Consumes a query but NOT useAuth (like DataExplorerPage). */
function Query({ fn }: { fn: () => Promise<string> }) {
	const q = useQuery({ queryKey: ['probe'], queryFn: fn })
	return <p>{q.isError ? 'error' : (q.data ?? 'loading')}</p>
}

function Sibling() {
	const auth = useAuth()
	return (
		<>
			<button onClick={() => auth.setToken('t1')}>save</button>
			<button onClick={() => auth.clearToken()}>clear</button>
		</>
	)
}

function setup(fn: () => Promise<string>) {
	const client = createQueryClient()
	const store = new TokenStore()
	render(
		<QueryClientProvider client={client}>
			<AuthProvider store={store}>
				<Query fn={fn} />
				<Sibling />
			</AuthProvider>
		</QueryClientProvider>,
	)
}

describe('AuthProvider refetch', { concurrent: false }, () => {
	it.each(['save', 'clear'])('refetches an active failed query on %s', async (name) => {
		const fn = vi
			.fn<() => Promise<string>>()
			.mockRejectedValueOnce(new Error('401'))
			.mockResolvedValue('ok')
		setup(fn)
		await screen.findByText('error')
		await userEvent.click(screen.getByRole('button', { name }))
		await waitFor(() => expect(fn).toHaveBeenCalledTimes(2))
		await screen.findByText('ok')
	})
})
