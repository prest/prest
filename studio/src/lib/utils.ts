import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Merge conditional class names and de-duplicate conflicting Tailwind utilities
 * (e.g. `px-2 px-4` → `px-4`). Used by every UI primitive.
 */
export function cn(...inputs: ClassValue[]): string {
	return twMerge(clsx(inputs))
}
