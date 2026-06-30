package adapters

import "net/http"

// CatalogQuerier builds SQL for listing databases, schemas, and tables.
type CatalogQuerier interface {
	DatabaseClause(req *http.Request) (query string, hasCount bool)
	DatabaseWhere(requestWhere string) (whereSyntax string)
	DatabaseOrderBy(order string, hasCount bool) (orderBy string)

	SchemaClause(req *http.Request) (query string, hasCount bool)
	SchemaOrderBy(order string, hasCount bool) (orderBy string)

	TableClause() (query string)
	TableWhere(requestWhere string) (whereSyntax string)
	TableOrderBy(order string) (orderBy string)

	SchemaTablesClause() (query string)
	SchemaTablesWhere(requestWhere string) (whereSyntax string)
	SchemaTablesOrderBy(order string) (orderBy string)
}
