/**
 * High-level pREST endpoint helpers built on {@link PrestClient}.
 *
 * Discovery endpoints (`/databases`, `/schemas`, `/tables`) treat 403/404 as
 * "feature disabled" rather than a hard error, so the Overview and Data
 * Explorer can degrade gracefully when catalog access is restricted.
 */

import type { PrestClient, ProbeResult } from '@/lib/api/client'
import { ApiError, toErrorMessage } from '@/lib/errors'
import { buildQueryString, showPath, tablePath, type QuerySpec } from '@/lib/api/url'
import type { DiscoveryResult, Row, StudioMeta, TableStructureColumn } from '@/lib/api/types'

export async function getMeta(client: PrestClient): Promise<StudioMeta> {
	const res = await client.requestJson<StudioMeta>('/_studio/api/meta', { noAuth: true })
	return res.data
}

/** GET /_health – status-only (empty body expected). */
export function getHealth(client: PrestClient): Promise<ProbeResult> {
	return client.probe('/_health', { noAuth: true })
}

/** GET /_ready – status-only (empty body expected). */
export function getReady(client: PrestClient): Promise<ProbeResult> {
	return client.probe('/_ready', { noAuth: true })
}

/** Probe the MCP endpoint with a plain GET to detect availability. */
export function probeMcp(client: PrestClient): Promise<ProbeResult> {
	return client.probe('/_mcp')
}

async function discover<T = Row>(
	client: PrestClient,
	path: string,
	query?: string,
): Promise<DiscoveryResult<T>> {
	try {
		const res = await client.requestJson<T[]>(path, { query })
		const items = Array.isArray(res.data) ? res.data : []
		return { items, disabled: false }
	} catch (err) {
		if (err instanceof ApiError && err.isDisabled) {
			return { disabled: true }
		}
		return { disabled: false, error: toErrorMessage(err) }
	}
}

export function listDatabases(client: PrestClient): Promise<DiscoveryResult> {
	return discover(client, '/databases')
}

export function listSchemas(client: PrestClient): Promise<DiscoveryResult> {
	return discover(client, '/schemas')
}

export function listTables(client: PrestClient): Promise<DiscoveryResult> {
	return discover(client, '/tables')
}

export async function showTable(
	client: PrestClient,
	database: string,
	schema: string,
	table: string,
): Promise<TableStructureColumn[]> {
	const res = await client.requestJson<TableStructureColumn[]>(showPath(database, schema, table))
	return Array.isArray(res.data) ? res.data : []
}

export interface QueryTableResult {
	rows: Row[]
	status: number
	durationMs: number
}

export async function queryTable(
	client: PrestClient,
	database: string,
	schema: string,
	table: string,
	spec: QuerySpec,
	signal?: AbortSignal,
): Promise<QueryTableResult> {
	const res = await client.requestJson<Row[]>(tablePath(database, schema, table), {
		query: buildQueryString(spec),
		signal,
	})
	return {
		rows: Array.isArray(res.data) ? res.data : [],
		status: res.status,
		durationMs: res.durationMs,
	}
}

/**
 * Extract a human-friendly name from a catalog record. pREST field names vary
 * (e.g. `datname`, `schema_name`, `table_name`); the first matching candidate
 * wins, otherwise the first string value is used.
 */
export function pickName(record: Row, candidates: string[]): string {
	for (const key of candidates) {
		const v = record[key]
		if (typeof v === 'string' && v.length > 0) return v
	}
	for (const v of Object.values(record)) {
		if (typeof v === 'string' && v.length > 0) return v
	}
	return ''
}

export const NAME_KEYS = {
	database: ['datname', 'database', 'name', 'db'],
	schema: ['schema_name', 'schema', 'name', 'nspname'],
	table: ['table_name', 'table', 'name', 'relname'],
} as const
