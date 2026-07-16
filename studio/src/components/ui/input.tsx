import * as React from 'react'
import { cn } from '@/lib/utils'

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
	({ className, type, ...props }, ref) => {
		return (
			<input
				ref={ref}
				type={type ?? 'text'}
				className={cn(
					'flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors',
					'placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
					'disabled:cursor-not-allowed disabled:opacity-50',
					className,
				)}
				{...props}
			/>
		)
	},
)
Input.displayName = 'Input'
