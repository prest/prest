/** Shapes for pREST responses consumed by Studio. Kept intentionally loose
 * because pREST field names vary by configuration; helpers normalize them. */

export interface StudioMeta {
	version: string
	commit: string
	buildDate: string
	apiBasePath: string
	mcpEndpoint: string
}

export type Row = Record<string, unknown>

export interface TableStructureColumn {
	column_name?: string
	data_type?: string
	is_nullable?: string
	[key: string]: unknown
}

export interface DiscoveryResult<T = Row> {
	/** Present when the endpoint responded successfully. */
	items?: T[]
	/** True when 403/404 indicated the feature is unavailable. */
	disabled: boolean
	/** Populated on unexpected (non-disabled) errors. */
	error?: string
}
