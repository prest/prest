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
)
