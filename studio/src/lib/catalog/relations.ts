/**
 * Pure helpers for the catalog tree, column order and Overview counts.
 *
 * `/tables` returns every relation (`type`: table | view | sequence | ...).
 * Only browsable relations in non-system schemas are shown to users.
 */
import { NAME_KEYS, pickName } from '@/lib/api/prest'
import type { Row, TableStructureColumn } from '@/lib/api/types'

const SYSTEM_SCHEMA = /^(pg_catalog|information_schema|pg_toast|pg_temp_\d+|pg_toast_temp_\d+)$/

const NON_BROWSABLE_TYPES = new Set(['sequence', 'index', 'partitioned index', 'composite type'])

export function isSystemSchema(name: string): boolean {
	return SYSTEM_SCHEMA.test(name)
}

/** A relation Studio can browse: not a sequence or index. A missing type is kept. */
export function isBrowsableRelation(record: Row): boolean {
	const type = record.type
	if (typeof type !== 'string') return true
	return !NON_BROWSABLE_TYPES.has(type.trim().toLowerCase())
}

function schemaOf(record: Row): string {
	return pickName(record, [...NAME_KEYS.schema]) || 'public'
}

/** Group browsable relations by non-system schema; schemas and names sorted. */
export function groupBySchema(items: Row[]): Map<string, string[]> {
	const groups = new Map<string, string[]>()
	for (const record of items) {
		if (!isBrowsableRelation(record)) continue
		const table = pickName(record, [...NAME_KEYS.table])
		if (!table) continue
		const schema = schemaOf(record)
		if (isSystemSchema(schema)) continue
		const list = groups.get(schema) ?? []
		list.push(table)
		groups.set(schema, list)
	}
	for (const list of groups.values()) list.sort()
	return new Map([...groups.entries()].sort(([a], [b]) => (a < b ? -1 : 1)))
}

/**
 * Order row keys by the table's column order (`position`, then `/show` order).
 * Keys unknown to the structure are appended in first-seen order.
 */
export function orderColumns(
	rowKeys: string[],
	structure: TableStructureColumn[] | undefined,
): string[] {
	const named = (structure ?? [])
		.map((c, i) => ({
			name: c.column_name,
			pos: typeof c.position === 'number' ? c.position : i,
			i,
		}))
		.filter((c): c is { name: string; pos: number; i: number } => typeof c.name === 'string')
		.sort((a, b) => a.pos - b.pos || a.i - b.i)
	const present = new Set(rowKeys)
	const ordered = named.map((c) => c.name).filter((n) => present.has(n))
	const seen = new Set(ordered)
	return [...ordered, ...rowKeys.filter((k) => !seen.has(k))]
}

/** Counts shown on Overview: non-system schemas and browsable relations. */
export function countCatalog(
	schemas: Row[] | undefined,
	tables: Row[] | undefined,
): { schemas: number | undefined; tables: number | undefined } {
	return {
		schemas: schemas?.filter((r) => !isSystemSchema(pickName(r, [...NAME_KEYS.schema]))).length,
		tables: tables?.filter(
			(r) =>
				isBrowsableRelation(r) && !isSystemSchema(schemaOf(r)) && pickName(r, [...NAME_KEYS.table]),
		).length,
	}
}
