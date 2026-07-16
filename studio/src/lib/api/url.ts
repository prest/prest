/**
 * URL, path-segment encoding, and pREST query-parameter construction.
 *
 * pREST data routes look like `/{database}/{schema}/{table}` and accept filter
 * params in the form `column=$op.value`, ordering via `_order`, and pagination
 * via `_page` / `_page_size`.
 */

export type FilterOp =
	| 'eq'
	| 'ne'
	| 'gt'
	| 'gte'
	| 'lt'
	| 'lte'
	| 'like'
	| 'ilike'
	| 'nlike'
	| 'in'
	| 'nin'
	| 'null'
	| 'notnull'
	| 'tsquery'

export const FILTER_OPS: readonly FilterOp[] = [
	'eq',
	'ne',
	'gt',
	'gte',
	'lt',
	'lte',
	'like',
	'ilike',
	'nlike',
	'in',
	'nin',
	'null',
	'notnull',
	'tsquery',
]

/** Operators that do not require a value (unary predicates). */
export const UNARY_OPS: readonly FilterOp[] = ['null', 'notnull']

export interface Filter {
	column: string
	op: FilterOp
	value?: string
}

export interface QuerySpec {
	filters?: Filter[]
	/** Order columns; prefix with `-` for descending, e.g. `-created_at`. */
	order?: string[]
	page?: number
	pageSize?: number
	/** Restrict selected columns via `_select`. */
	select?: string[]
	/** Extra raw params merged last (values are encoded). */
	extra?: Record<string, string>
}

/**
 * Encode a single path segment. Unlike `encodeURIComponent`, forward slashes
 * are always escaped so a segment can never inject an extra path level.
 */
export function encodeSegment(segment: string): string {
	return encodeURIComponent(segment)
}

/** Join and encode ordered path segments into a leading-slash path. */
export function buildPath(...segments: Array<string | number>): string {
	const encoded = segments
		.map((s) => String(s))
		.filter((s) => s.length > 0)
		.map((s) => encodeSegment(s))
	return '/' + encoded.join('/')
}

/** Build the data path for a table: `/{db}/{schema}/{table}`. */
export function tablePath(database: string, schema: string, table: string): string {
	return buildPath(database, schema, table)
}

/** Build the structural `/show/{db}/{schema}/{table}` path. */
export function showPath(database: string, schema: string, table: string): string {
	return buildPath('show', database, schema, table)
}

function filterToParam(f: Filter): [string, string] | null {
	if (!f.column) return null
	if (UNARY_OPS.includes(f.op)) {
		return [f.column, `$${f.op}`]
	}
	const value = f.value ?? ''
	return [f.column, `$${f.op}.${value}`]
}

/**
 * Construct a `URLSearchParams` from a {@link QuerySpec}. Values are encoded by
 * URLSearchParams; column names are used as keys as-is (pREST expects them raw).
 */
export function buildSearchParams(spec: QuerySpec): URLSearchParams {
	const params = new URLSearchParams()

	for (const f of spec.filters ?? []) {
		const kv = filterToParam(f)
		if (kv) params.append(kv[0], kv[1])
	}

	if (spec.select && spec.select.length > 0) {
		params.set('_select', spec.select.join(','))
	}

	if (spec.order && spec.order.length > 0) {
		params.set('_order', spec.order.join(','))
	}

	if (typeof spec.page === 'number' && spec.page > 0) {
		params.set('_page', String(spec.page))
	}

	if (typeof spec.pageSize === 'number' && spec.pageSize > 0) {
		params.set('_page_size', String(spec.pageSize))
	}

	for (const [k, v] of Object.entries(spec.extra ?? {})) {
		params.set(k, v)
	}

	return params
}

/** Build a query string (without leading `?`) from a {@link QuerySpec}. */
export function buildQueryString(spec: QuerySpec): string {
	return buildSearchParams(spec).toString()
}

/** Combine a path and query spec into a relative URL. */
export function buildUrl(path: string, spec?: QuerySpec): string {
	if (!spec) return path
	const qs = buildQueryString(spec)
	return qs ? `${path}?${qs}` : path
}
