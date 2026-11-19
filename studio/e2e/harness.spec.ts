import { expect, test } from '@playwright/test'

import { AUTH_ORIGIN, MAIN_ORIGIN, STACK_MODE } from './support/env'
import { signToken } from './support/jwt'

// Guards the harness itself: both prestd instances are up and the auth instance really enforces JWT.
test.skip(!STACK_MODE, 'Stack mode only (STUDIO_E2E_STACK=1).')

test('main instance is ready and open', async ({ request }) => {
	expect((await request.get(`${MAIN_ORIGIN}/_ready`)).status()).toBe(200)
	const res = await request.get(`${MAIN_ORIGIN}/databases`)
	expect(res.status()).toBe(200)
	expect(await res.json()).toEqual([{ datname: 'prest' }])
})

test('auth instance enforces JWT (401 without token, 200 with)', async ({ request }) => {
	expect((await request.get(`${AUTH_ORIGIN}/_ready`)).status()).toBe(200)
	expect((await request.get(`${AUTH_ORIGIN}/tables`)).status()).toBe(401)
	const res = await request.get(`${AUTH_ORIGIN}/tables`, {
		headers: { Authorization: `Bearer ${signToken()}` },
	})
	expect(res.status()).toBe(200)
})
