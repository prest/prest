import * as React from 'react'
import { Check, Copy } from 'lucide-react'
import { Button, type ButtonProps } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface CopyButtonProps extends Omit<ButtonProps, 'onClick' | 'children'> {
	value: string
	label?: string
}

/** Copy `value` to the clipboard, showing transient confirmation feedback. */
export function CopyButton({
	value,
	label = 'Copy',
	variant = 'outline',
	size = 'sm',
	className,
	...props
}: CopyButtonProps) {
	const [copied, setCopied] = React.useState(false)

	const onCopy = React.useCallback(async () => {
		try {
			if (!navigator.clipboard?.writeText) return
			await navigator.clipboard.writeText(value)
			setCopied(true)
			setTimeout(() => setCopied(false), 1500)
		} catch {
			/* clipboard may be unavailable; fail silently */
		}
	}, [value])

	return (
		<Button
			variant={variant}
			size={size}
			className={cn(className)}
			onClick={onCopy}
			aria-label={label}
			{...props}
		>
			{copied ? <Check className="text-success" /> : <Copy />}
			{label}
		</Button>
	)
}
