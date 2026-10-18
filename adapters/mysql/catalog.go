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
	if served := a.servedDatabases(); served != nil {
		where += " AND " + servedFilter("datname", served)
	}
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
	// Schemas are databases on MySQL: without a registry list only the served
	// ones. The filter sits inside the derived table, so the controller's
	// appended " WHERE <request>" is unaffected.
	filter := ""
	if served := a.servedDatabases(); served != nil {
		filter = " AND " + servedFilter("SCHEMA_NAME", served)
	} else if reg := a.registryPhysicalNames(); reg != nil {
		// Registry mode: /schemas must not list databases no entry serves.
		filter = " AND " + servedFilter("SCHEMA_NAME", reg)
	}
	return fmt.Sprintf(statements.SchemasFrom, field, filter), hasCount
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

// TableWhere limits table listings to the served databases' tables; the
// MySQL user may see databases pREST does not serve.
func (a *Adapter) TableWhere(requestWhere string) string {
	where := "WHERE 1=1 AND " + a.tableScope()
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

// tableScope pins the `schema` column of TablesFrom: a registry entry serves
// only its physical database, otherwise pg.database plus pg.allowed_databases.
func (a *Adapter) tableScope() string {
	if a.cfg != nil && a.cfg.HasDatabaseRegistry() {
		return "`schema` = " + quoteLiteral(a.PhysicalName(""))
	}
	return servedFilter("`schema`", a.servedDatabases())
}

// registryPhysicalNames lists the physical database of every registry entry
// (entry database, else pg.database, else alias); nil without a registry.
func (a *Adapter) registryPhysicalNames() []string {
	if a.cfg == nil || !a.cfg.HasDatabaseRegistry() {
		return nil
	}
	names := make([]string, 0, len(a.cfg.Databases))
	for _, db := range a.cfg.Databases {
		name := db.Database
		if name == "" {
			name = a.cfg.PGDatabase
		}
		if name == "" {
			name = db.Alias
		}
		names = append(names, name)
	}
	return dedupeNonEmpty(names)
}
