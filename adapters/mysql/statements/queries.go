package statements

// SystemSchemas are hidden from catalog listings. Hiding them is not an ACL.
const SystemSchemas = `'mysql','information_schema','performance_schema','sys'`

const (
	DatabasesFrom = `
SELECT %s FROM (
  SELECT SCHEMA_NAME AS datname
  FROM information_schema.schemata
  WHERE SCHEMA_NAME NOT IN (` + SystemSchemas + `)
) d`

	SchemasFrom = `
SELECT %s FROM (
  SELECT SCHEMA_NAME AS schema_name
  FROM information_schema.schemata
  WHERE SCHEMA_NAME NOT IN (` + SystemSchemas + `)
) s`

	TablesFrom = `
SELECT ` + "`schema`, `name`, `type`, `owner`" + ` FROM (
  SELECT
    TABLE_SCHEMA AS ` + "`schema`" + `,
    TABLE_NAME AS ` + "`name`" + `,
    CASE TABLE_TYPE
      WHEN 'BASE TABLE' THEN 'table'
      WHEN 'VIEW' THEN 'view'
    END AS ` + "`type`" + `,
    NULL AS ` + "`owner`" + `
  FROM information_schema.tables
  WHERE TABLE_SCHEMA NOT IN (` + SystemSchemas + `)
    AND TABLE_TYPE IN ('BASE TABLE', 'VIEW')
) t`

	SchemaTablesFrom = `
SELECT TABLE_NAME AS name, TABLE_SCHEMA AS ` + "`schema`" + `, ? AS ` + "`database`" + `
FROM information_schema.tables`

	SchemaTablesWhere = ` WHERE TABLE_SCHEMA = ? AND TABLE_TYPE IN ('BASE TABLE', 'VIEW')`

	ShowColumns = `
SELECT
  TABLE_SCHEMA AS table_schema,
  TABLE_NAME AS table_name,
  ORDINAL_POSITION AS position,
  COLUMN_NAME AS column_name,
  DATA_TYPE AS data_type,
  COALESCE(CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION) AS max_length,
  IS_NULLABLE AS is_nullable,
  CASE
    WHEN EXTRA LIKE '%VIRTUAL GENERATED%' OR EXTRA LIKE '%STORED GENERATED%' THEN 'ALWAYS'
    ELSE 'NEVER'
  END AS is_generated,
  'YES' AS is_updatable,
  COLUMN_DEFAULT AS default_value
FROM information_schema.columns`

	ShowTableWhere = `
WHERE TABLE_NAME = ? AND TABLE_SCHEMA = ?
ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION`

	ShowColumnsWhere = `
WHERE TABLE_SCHEMA NOT IN (` + SystemSchemas + `)
ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION`

	PKColumns = `
SELECT COLUMN_NAME, EXTRA
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_KEY = 'PRI'
ORDER BY ORDINAL_POSITION`

	TableExists = `
SELECT 1 FROM information_schema.TABLES
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
LIMIT 1`
)
