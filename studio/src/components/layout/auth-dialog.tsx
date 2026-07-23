import * as React from 'react'
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuth } from '@/app/providers'

interface AuthDialogProps {
	open: boolean
	onOpenChange: (open: boolean) => void
}

/**
 * Collects a bearer token. "Remember for this tab" opts into sessionStorage
 * (never localStorage); otherwise the token is held in memory only.
 */
export function AuthDialog({ open, onOpenChange }: AuthDialogProps) {
	const { token, remembered, setToken, clearToken } = useAuth()
	const [value, setValue] = React.useState('')
	const [remember, setRemember] = React.useState(remembered)

	// Repopulate fields from the current token each time the dialog opens.
	// Done during render (not in an effect) per React's state-adjustment guidance.
	const [prevOpen, setPrevOpen] = React.useState(open)
	if (open !== prevOpen) {
		setPrevOpen(open)
		if (open) {
			setValue(token ?? '')
			setRemember(remembered)
		}
	}

	const onSubmit = (e: React.FormEvent) => {
		e.preventDefault()
		const trimmed = value.trim()
		if (!trimmed) {
			clearToken()
		} else {
			setToken(trimmed, remember)
		}
		onOpenChange(false)
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<form onSubmit={onSubmit}>
					<DialogHeader>
						<DialogTitle>Bearer token</DialogTitle>
						<DialogDescription>
							Sent as <code className="font-mono">Authorization: Bearer …</code> on API and MCP
							requests. It is never written to <code className="font-mono">localStorage</code>.
						</DialogDescription>
					</DialogHeader>

					<div className="my-4 flex flex-col gap-3">
						<div className="flex flex-col gap-1.5">
							<Label htmlFor="token-input">Token</Label>
							<Input
								id="token-input"
								type="password"
								autoComplete="off"
								placeholder="eyJhbGciOi…"
								value={value}
								onChange={(e) => setValue(e.target.value)}
								autoFocus
							/>
						</div>

						<label className="flex items-center gap-2 text-sm text-muted-foreground">
							<input
								type="checkbox"
								className="size-4 accent-primary"
								checked={remember}
								onChange={(e) => setRemember(e.target.checked)}
							/>
							Remember for this tab (sessionStorage)
						</label>
					</div>

					<DialogFooter>
						{token ? (
							<Button
								type="button"
								variant="ghost"
								onClick={() => {
									clearToken()
									onOpenChange(false)
								}}
							>
								Clear token
							</Button>
						) : null}
						<Button type="submit">Save</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	)
}
