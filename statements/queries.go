package statements

const (
	// Databases list all data bases
	Databases = `
SELECT
	datname
FROM
	pg_database
WHERE
	NOT datistemplate
ORDER BY
	datname ASC`

	// Schemas list all schema on data base
	Schemas = `
SELECT
	schema_name
FROM
	information_schema.schemata
ORDER BY
	schema_name ASC`

	// Tables list all tables
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
	TablesWhere = `
WHERE
	c.relkind IN ('r','v','m','S','s','') AND
	n.nspname !~ '^pg_toast' AND
	n.nspname NOT IN ('information_schema', 'pg_catalog') AND
	has_schema_privilege(n.nspname, 'USAGE') `
	TablesOrderBy = `
ORDER BY 1, 2`
	Tables = TablesSelect + TablesWhere + TablesOrderBy
)
