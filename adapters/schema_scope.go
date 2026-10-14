package adapters

// SchemaScoper limits which {schema} path values an adapter serves for the
// selected {database}. Engines whose schemas are databases (MySQL) pin
// schema to database.
type SchemaScoper interface {
	AllowsSchema(database, schema string) bool
}
