package adapters

// Dialect exposes engine-specific identifier quoting and bind placeholders
// for handlers that assemble SQL outside the adapter (auth lookup, MCP select).
type Dialect interface {
	// QuoteIdentifier validates and quotes a name; dotted names are quoted per part.
	QuoteIdentifier(name string) (string, error)
	// Placeholder returns the bind marker for the 1-based position.
	Placeholder(position int) string
}
