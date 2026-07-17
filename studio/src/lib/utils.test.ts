import { describe, expect, it } from 'vitest'
import { cn } from '@/lib/utils'

describe('cn', () => {
	it('joins truthy class names', () => {
		expect(cn('a', 'b')).toBe('a b')
	})

	it('ignores falsy values', () => {
		expect(cn('a', false, null, undefined, '', 'b')).toBe('a b')
	})

	it('supports conditional objects and arrays', () => {
		expect(cn('a', { b: true, c: false }, ['d'])).toBe('a b d')
	})

	it('merges conflicting tailwind utilities (last wins)', () => {
		expect(cn('px-2', 'px-4')).toBe('px-4')
		expect(cn('text-sm', 'text-lg')).toBe('text-lg')
	})
})
