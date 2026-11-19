const DECIMAL = /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/
const INTEGER = /^[+-]?\d+$/

type Coercer = (name: string, raw: string) => unknown

const INTEGER_LITERAL = /^-?\d+$/

/**
 * Parse form JSON. Integer literals outside the safe range would be rounded
 * when the request is serialized, so they are rejected instead.
 * `context.source` (when the runtime provides it) tells `1e300` from an integer literal.
 */
function parseJsonOrThrow(name: string, raw: string, message: string): unknown {
	let unsafe = false
	let parsed: unknown
	try {
		parsed = JSON.parse(raw, (_key: string, value: unknown, context?: { source?: string }) => {
			if (typeof value !== 'number' || Number.isSafeInteger(value)) return value
			const integer =
				context?.source === undefined
					? Number.isInteger(value)
					: INTEGER_LITERAL.test(context.source)
			if (integer) unsafe = true
			return value
		})
	} catch {
		throw new Error(message)
	}
	if (unsafe) {
		throw new Error(`Field "${name}" has an integer beyond ±9007199254740991; send it as a string.`)
	}
	return parsed
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
		const parsed = parseJsonOrThrow(name, trimmed, message)
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
	const parsed = parseJsonOrThrow(name, raw, message)
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
