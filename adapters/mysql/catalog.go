package mysql

import (
	"fmt"
	"net/http"

	"github.com/prest/prest/v2/adapters/mysql/statements"
)

func (a *Adapter) DatabaseClause(req *http.Request) (string, bool) {
	field := "datname"
	hasCount := false
	if req != nil && req.URL.Query().Get("_count") != "" {
		field = "COUNT(datname)"
		hasCount = true
	}
	return fmt.Sprintf(statements.DatabasesFrom, field), hasCount
}

func (a *Adapter) DatabaseWhere(requestWhere string) string {
	where := " WHERE 1=1"
	if requestWhere != "" {
		where += " AND " + requestWhere
	}
	return where
}

func (a *Adapter) DatabaseOrderBy(order string, hasCount bool) string {
	if order != "" {
		return order
	}
	if hasCount {
		return ""
	}
	return " ORDER BY datname ASC"
}

func (a *Adapter) SchemaClause(req *http.Request) (string, bool) {
	field := "schema_name"
	hasCount := false
	if req != nil && req.URL.Query().Get("_count") != "" {
		field = "COUNT(schema_name)"
		hasCount = true
	}
	return fmt.Sprintf(statements.SchemasFrom, field), hasCount
}

func (a *Adapter) SchemaOrderBy(order string, hasCount bool) string {
	if order != "" {
		return order
	}
	if hasCount {
		return ""
	}
	return " ORDER BY schema_name ASC"
}

func (a *Adapter) TableClause() string { return statements.TablesFrom }

func (a *Adapter) TableWhere(requestWhere string) string {
	where := "WHERE 1=1"
	if requestWhere != "" {
		where += " AND " + requestWhere
	}
	return where
}

func (a *Adapter) TableOrderBy(order string) string {
	if order != "" {
		return order
	}
	return " ORDER BY `schema`, `name`"
}

func (a *Adapter) SchemaTablesClause() string { return statements.SchemaTablesFrom }

func (a *Adapter) SchemaTablesWhere(requestWhere string) string {
	where := statements.SchemaTablesWhere
	if requestWhere != "" {
		where += " AND " + requestWhere
	}
	return where
}

func (a *Adapter) SchemaTablesOrderBy(order string) string {
	if order != "" {
		return order
	}
	return " ORDER BY `name` ASC"
}
