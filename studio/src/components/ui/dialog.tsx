import * as React from 'react'
import * as DialogPrimitive from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'

export const Dialog = DialogPrimitive.Root
export const DialogTrigger = DialogPrimitive.Trigger
export const DialogClose = DialogPrimitive.Close
export const DialogPortal = DialogPrimitive.Portal

export const DialogOverlay = React.forwardRef<
	React.ComponentRef<typeof DialogPrimitive.Overlay>,
	React.ComponentPropsWithoutRef<typeof DialogPrimitive.Overlay>
>(({ className, ...props }, ref) => (
	<DialogPrimitive.Overlay
		ref={ref}
		className={cn('fixed inset-0 z-50 bg-black/50 backdrop-blur-sm', className)}
		{...props}
	/>
))
DialogOverlay.displayName = DialogPrimitive.Overlay.displayName

export const DialogContent = React.forwardRef<
	React.ComponentRef<typeof DialogPrimitive.Content>,
	React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content>
>(({ className, children, ...props }, ref) => (
	<DialogPortal>
		<DialogOverlay />
		<DialogPrimitive.Content
			ref={ref}
			className={cn(
				'fixed left-1/2 top-1/2 z-50 grid w-full max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4',
				'rounded-lg border border-border bg-card p-6 text-card-foreground shadow-lg',
				'focus:outline-none',
				className,
			)}
			{...props}
		>
			{children}
			<DialogPrimitive.Close
				className="absolute right-4 top-4 rounded-sm opacity-70 transition-opacity hover:opacity-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
				aria-label="Close"
			>
				<X className="size-4" />
			</DialogPrimitive.Close>
		</DialogPrimitive.Content>
	</DialogPortal>
))
DialogContent.displayName = DialogPrimitive.Content.displayName

export function DialogHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
	return <div className={cn('flex flex-col gap-1.5 text-left', className)} {...props} />
}

export function DialogFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
	return (
		<div
			className={cn('flex flex-col-reverse gap-2 sm:flex-row sm:justify-end', className)}
			{...props}
		/>
	)
}

export const DialogTitle = React.forwardRef<
	React.ComponentRef<typeof DialogPrimitive.Title>,
	React.ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(({ className, ...props }, ref) => (
	<DialogPrimitive.Title
		ref={ref}
		className={cn('text-lg font-semibold leading-none', className)}
		{...props}
	/>
))
DialogTitle.displayName = DialogPrimitive.Title.displayName

export const DialogDescription = React.forwardRef<
	React.ComponentRef<typeof DialogPrimitive.Description>,
	React.ComponentPropsWithoutRef<typeof DialogPrimitive.Description>
>(({ className, ...props }, ref) => (
	<DialogPrimitive.Description
		ref={ref}
		className={cn('text-sm text-muted-foreground', className)}
		{...props}
	/>
))
DialogDescription.displayName = DialogPrimitive.Description.displayName
