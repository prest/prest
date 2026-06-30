package adapters

// SQLBuilder assembles CRUD SQL statements.
type SQLBuilder interface {
	SelectFields(fields []string) (sql string, err error)
	SelectSQL(selectStr string, database string, schema string, table string) string
	InsertSQL(database string, schema string, table string, names string, placeholders string) string
	UpdateSQL(database string, schema string, table string, setSyntax string) string
	DeleteSQL(database string, schema string, table string) string
}
