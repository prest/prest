import * as React from 'react'
import { cn } from '@/lib/utils'

export function Card({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
	return (
		<div
			className={cn('rounded-lg border border-border bg-card text-card-foreground', className)}
			{...props}
		/>
	)
}

export function CardHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
	return <div className={cn('flex flex-col gap-1 p-4', className)} {...props} />
}

export function CardTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
	return <h3 className={cn('font-semibold leading-none', className)} {...props} />
}

export function CardDescription({
	className,
	...props
}: React.HTMLAttributes<HTMLParagraphElement>) {
	return <p className={cn('text-sm text-muted-foreground', className)} {...props} />
}

export function CardContent({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
	return <div className={cn('p-4 pt-0', className)} {...props} />
}
