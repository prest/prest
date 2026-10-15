package adapters

// SchemaScoper limits which {schema} path values an adapter serves. Engines
// where {schema} selects a whole database (MySQL with [[databases]]) pin it.
type SchemaScoper interface {
	AllowsSchema(schema string) bool
}

// DatabaseSchemaScoper is SchemaScoper with the routed {database}: engines
// whose schemas are databases (MySQL) pin {schema} to it. Checked before
// SchemaScoper.
type DatabaseSchemaScoper interface {
	AllowsDatabaseSchema(database, schema string) bool
}
