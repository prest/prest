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
)
