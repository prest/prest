/**
 * Lossless JSON helpers: numbers that do not round-trip through a JS double
 * (e.g. `9007199254740993`, high-precision numerics) keep their source text.
 */

/**
 * Parse JSON text, preserving the exact source literal of numbers that would
 * lose precision as a double.
 */
export function parseJsonLossless(text: string): unknown {
	return JSON.parse(text, losslessReviver)
}

/**
 * `JSON.parse` reviver backing {@link parseJsonLossless}. `context.source` is
 * the raw literal text when the runtime provides it.
 */
export function losslessReviver(
	this: unknown,
	key: string,
	value: unknown,
	context?: { source?: string },
): unknown {
	void key
	if (
		typeof value === 'number' &&
		typeof context?.source === 'string' &&
		String(value) !== context.source
	) {
		return context.source
	}
	return value
}

const TOKEN_RE = /"(?:[^"\\]|\\.)*"|[{}[\],:]|[^\s{}[\],:"]+/g
const CLOSER: Record<string, string> = { '{': '}', '[': ']' }

/** Split valid JSON text into string, punctuation and literal tokens verbatim. */
function tokenize(text: string): string[] {
	return text.match(TOKEN_RE) ?? []
}

/**
 * Pretty-print JSON text without altering number literals. `indent` defaults
 * to 2. Invalid JSON is returned unchanged.
 */
export function formatJsonText(text: string, indent = 2): string {
	try {
		JSON.parse(text)
	} catch {
		return text
	}
	const tokens = tokenize(text)
	const nl = (depth: number) => '\n' + ' '.repeat(indent * depth)
	let out = ''
	let depth = 0
	for (let i = 0; i < tokens.length; i++) {
		const t = tokens[i]!
		if (t in CLOSER) {
			if (tokens[i + 1] === CLOSER[t]) {
				out += t + tokens[++i]
			} else {
				out += t + nl(++depth)
			}
		} else if (t === '}' || t === ']') {
			out += nl(--depth) + t
		} else if (t === ',') {
			out += ',' + nl(depth)
		} else {
			out += t === ':' ? ': ' : t
		}
	}
	return out
}
