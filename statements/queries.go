package statements

const (
	// Databases list all data bases

	// DatabasesSelect clause
	DatabasesSelect = `
SELECT
	datname
FROM
	pg_database`
	// DatabasesWhere clause
	DatabasesWhere = `
WHERE
	NOT datistemplate`
	// DatabasesOrderBy clause
	DatabasesOrderBy = `
ORDER BY
	datname ASC`
	// Databases default query
	Databases = DatabasesSelect + DatabasesWhere + DatabasesOrderBy
	// Schemas list all schema on data base

	// SchemasSelect clause
	SchemasSelect = `
SELECT
	schema_name
FROM
	information_schema.schemata`
	// SchemasOrderBy clause
	SchemasOrderBy = `
ORDER BY
	schema_name ASC`
	// Schemas default query
	Schemas = SchemasSelect + SchemasOrderBy

	// Tables list all tables

	// TablesSelect clause
	TablesSelect = `
SELECT
	n.nspname as "schema",
	c.relname as "name",
	CASE c.relkind
		WHEN 'r' THEN 'table'
		WHEN 'v' THEN 'view'
		WHEN 'm' THEN 'materialized_view'
		WHEN 'i' THEN 'index'
		WHEN 'S' THEN 'sequence'
		WHEN 's' THEN 'special'
		WHEN 'f' THEN 'foreign_table'
	END as "type",
	pg_catalog.pg_get_userbyid(c.relowner) as "owner"
FROM
	pg_catalog.pg_class c
LEFT JOIN
	pg_catalog.pg_namespace n ON n.oid = c.relnamespace `
	// TablesWhere clause
	TablesWhere = `
WHERE
	c.relkind IN ('r','v','m','S','s','') AND
	n.nspname !~ '^pg_toast' AND
	n.nspname NOT IN ('information_schema', 'pg_catalog') AND
	has_schema_privilege(n.nspname, 'USAGE') `
	// TablesOrderBy clause
	TablesOrderBy = `
ORDER BY 1, 2`
	// Tables default query
	Tables = TablesSelect + TablesWhere + TablesOrderBy
	// list all tables in schema and database

	// SchemaTablesSelect clause
	SchemaTablesSelect = `
SELECT
	t.tablename as "name",
	t.schemaname as "schema",
	sc.catalog_name as "database"
FROM
	pg_catalog.pg_tables t
INNER JOIN
	information_schema.schemata sc ON sc.schema_name = t.schemaname`

	// SchemaTablesWhere clause
	SchemaTablesWhere = `
WHERE
	sc.catalog_name = $1 AND
	t.schemaname = $2`

	// SchemaTablesOrderBy clause
	SchemaTablesOrderBy = `
ORDER BY
	t.tablename ASC`

	// SchemaTables defalt query
	SchemaTables = SchemaTablesSelect + SchemaTablesWhere + SchemaTablesOrderBy
)
