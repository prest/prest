package adapters

// SchemaScoper limits which {schema} path values an adapter serves. Engines
// where {schema} selects a whole database (MySQL with [[databases]]) pin it.
type SchemaScoper interface {
	AllowsSchema(schema string) bool
}
