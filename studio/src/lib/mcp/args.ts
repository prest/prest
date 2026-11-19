const DECIMAL = /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/
const INTEGER = /^[+-]?\d+$/

type Coercer = (name: string, raw: string) => unknown

function parseJsonOrThrow(raw: string, message: string): unknown {
	try {
		return JSON.parse(raw)
	} catch {
		throw new Error(message)
	}
}

function coerceNumber(name: string, raw: string): number {
	const text = raw.trim()
	const n = DECIMAL.test(text) ? Number(text) : NaN
	if (!Number.isFinite(n)) throw new Error(`Field "${name}" must be a number.`)
	return n
}

function coerceInteger(name: string, raw: string): number {
	const text = raw.trim()
	const n = INTEGER.test(text) ? Number(text) : NaN
	if (!Number.isSafeInteger(n)) throw new Error(`Field "${name}" must be an integer.`)
	return n
}

function coerceBoolean(name: string, raw: string): boolean {
	const v = raw.trim()
	if (v === 'true') return true
	if (v === 'false') return false
	throw new Error(`Field "${name}" must be true or false.`)
}

function coerceArray(name: string, raw: string): unknown[] {
	const trimmed = raw.trim()
	if (trimmed.startsWith('[')) {
		const message = `Field "${name}" must be a JSON array.`
		const parsed = parseJsonOrThrow(trimmed, message)
		if (!Array.isArray(parsed)) throw new Error(message)
		return parsed
	}
	return trimmed
		.split(',')
		.map((part) => part.trim())
		.filter((part) => part !== '')
}

function coerceObject(name: string, raw: string): object {
	const message = `Field "${name}" must be a JSON object.`
	const parsed = parseJsonOrThrow(raw, message)
	if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error(message)
	return parsed
}

const COERCERS: Record<string, Coercer> = {
	number: coerceNumber,
	integer: coerceInteger,
	boolean: coerceBoolean,
	array: coerceArray,
	object: coerceObject,
}

/** Coerce a form-mode string into the JSON type the tool schema declares. */
export function coerceArg(name: string, raw: string, type?: string): unknown {
	const coercer = type !== undefined && Object.hasOwn(COERCERS, type) ? COERCERS[type] : undefined
	return coercer ? coercer(name, raw) : raw
}

/** Build tool arguments from form values, enforcing required fields. */
export function buildFormArgs(
	props: [string, { type?: string }][],
	required: Set<string>,
	formValues: Record<string, string>,
): Record<string, unknown> {
	const args: Record<string, unknown> = {}
	for (const [name, prop] of props) {
		const raw = formValues[name]
		if (raw !== undefined && raw.trim() !== '') args[name] = coerceArg(name, raw, prop.type)
	}
	const missing = [...required].filter((name) => !(name in args))
	if (missing.length > 0) {
		throw new Error(`Missing required fields: ${missing.join(', ')}`)
	}
	return args
}
