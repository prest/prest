package postgres

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prest/prest/v2/pkg/adapters"
	"github.com/prest/prest/v2/internal/postgres/statements"
)

// DatabaseClause return a SELECT `query`
func (adapter *postgres) DatabaseClause(req *http.Request) (query string, hasCount bool) {
	queries := req.URL.Query()
	countQuery := queries.Get("_count")

	query = fmt.Sprintf(statements.DatabasesSelect, statements.FieldDatabaseName)
	if countQuery != "" {
		hasCount = true
		query = fmt.Sprintf(statements.DatabasesSelect, statements.FieldCountDatabaseName)
	}
	return
}

// SchemaClause return a SELECT `query`
func (adapter *postgres) SchemaClause(req *http.Request) (query string, hasCount bool) {
	queries := req.URL.Query()
	countQuery := queries.Get("_count")

	query = fmt.Sprintf(statements.SchemasSelect, statements.FieldSchemaName)
	if countQuery != "" {
		hasCount = true
		query = fmt.Sprintf(statements.SchemasSelect, statements.FieldCountSchemaName)
	}
	return
}

// tableReference returns a quoted table identifier for SQL generation.
// With a database registry, the connection is already scoped to the physical
// database so only schema.table is qualified. Legacy mode uses database.schema.table.
func (adapter *postgres) tableReference(database, schema, table string) string {
	if adapter.cfg.HasDatabaseRegistry() {
		return fmt.Sprintf(`"%s"."%s"`, schema, table)
	}
	return fmt.Sprintf(`"%s"."%s"."%s"`, database, schema, table)
}

// SelectSQL generate select sql
func (adapter *postgres) SelectSQL(selectStr string, database string, schema string, table string) string {
	return fmt.Sprintf(`%s %s`, selectStr, adapter.tableReference(database, schema, table))
}

// InsertSQL generate insert sql
func (adapter *postgres) InsertSQL(database string, schema string, table string, names string, placeholders string) string {
	return fmt.Sprintf(`INSERT INTO %s(%s) VALUES%s`, adapter.tableReference(database, schema, table), names, placeholders)
}

// DeleteSQL generate delete sql
func (adapter *postgres) DeleteSQL(database string, schema string, table string) string {
	return fmt.Sprintf(`DELETE FROM %s`, adapter.tableReference(database, schema, table))
}

// UpdateSQL generate update sql
func (adapter *postgres) UpdateSQL(database string, schema string, table string, setSyntax string) string {
	return fmt.Sprintf(`UPDATE %s SET %s`, adapter.tableReference(database, schema, table), setSyntax)
}

// DatabaseWhere generate database where syntax
func (adapter *postgres) DatabaseWhere(requestWhere string) (whereSyntax string) {
	whereSyntax = statements.DatabasesWhere
	if requestWhere != "" {
		whereSyntax = fmt.Sprint(whereSyntax, " AND ", requestWhere)
	}
	return
}

// DatabaseOrderBy generate database order by
func (adapter *postgres) DatabaseOrderBy(order string, hasCount bool) (orderBy string) {
	if order != "" {
		orderBy = order
	} else if !hasCount {
		orderBy = fmt.Sprintf(statements.DatabasesOrderBy, statements.FieldDatabaseName)
	}
	return
}

// SchemaOrderBy generate schema order by
func (adapter *postgres) SchemaOrderBy(order string, hasCount bool) (orderBy string) {
	if order != "" {
		orderBy = order
	} else if !hasCount {
		orderBy = fmt.Sprintf(statements.SchemasOrderBy, statements.FieldSchemaName)
	}
	return
}

// TableClause generate table clause
func (adapter *postgres) TableClause() (query string) {
	query = statements.TablesSelect
	return
}

// TableWhere generate table where syntax
func (adapter *postgres) TableWhere(requestWhere string) (whereSyntax string) {
	whereSyntax = statements.TablesWhere
	if requestWhere != "" {
		whereSyntax = fmt.Sprint(whereSyntax, " AND ", requestWhere)
	}
	return
}

// TableOrderBy generate table order by
func (adapter *postgres) TableOrderBy(order string) (orderBy string) {
	if order != "" {
		orderBy = order
	} else {
		orderBy = statements.TablesOrderBy
	}
	return
}

// SchemaTablesClause generate schema tables clause
func (adapter *postgres) SchemaTablesClause() (query string) {
	query = statements.SchemaTablesSelect
	return
}

// SchemaTablesWhere generate schema tables where syntax
func (adapter *postgres) SchemaTablesWhere(requestWhere string) (whereSyntax string) {
	whereSyntax = statements.SchemaTablesWhere
	if requestWhere != "" {
		whereSyntax = fmt.Sprint(whereSyntax, " AND ", requestWhere)
	}
	return
}

// SchemaTablesOrderBy generate schema tables order by
func (adapter *postgres) SchemaTablesOrderBy(order string) (orderBy string) {
	if order != "" {
		orderBy = order
	} else {
		orderBy = statements.SchemaTablesOrderBy
	}
	return
}

// ShowTable shows table structure
func (adapter *postgres) ShowTable(schema, table string) adapters.Scanner {
	query := `SELECT table_schema, table_name, ordinal_position as position, column_name,data_type,
			  	CASE WHEN character_maximum_length is not null
					THEN character_maximum_length
					ELSE numeric_precision end as max_length,
			  	is_nullable,
			  	is_generated,
			  	is_updatable,
			  	column_default as default_value
			 FROM information_schema.columns
			 WHERE table_name=$1 AND table_schema=$2
			 ORDER BY table_schema, table_name, ordinal_position`
	return adapter.Query(query, table, schema)
}

// ShowTableCtx shows table structure
func (adapter *postgres) ShowTableCtx(ctx context.Context, schema, table string) adapters.Scanner {
	query := `SELECT table_schema, table_name, ordinal_position as position, column_name,data_type,
			  	CASE WHEN character_maximum_length is not null
					THEN character_maximum_length
					ELSE numeric_precision end as max_length,
			  	is_nullable,
			  	is_generated,
			  	is_updatable,
			  	column_default as default_value
			 FROM information_schema.columns
			 WHERE table_name=$1 AND table_schema=$2
			 ORDER BY table_schema, table_name, ordinal_position`
	return adapter.QueryCtx(ctx, query, table, schema)
}

// ShowColumnsCtx shows all user table structures in the current database.
func (adapter *postgres) ShowColumnsCtx(ctx context.Context) adapters.Scanner {
	query := `SELECT table_schema, table_name, ordinal_position as position, column_name,data_type,
			  	CASE WHEN character_maximum_length is not null
					THEN character_maximum_length
					ELSE numeric_precision end as max_length,
			  	is_nullable,
			  	is_generated,
			  	is_updatable,
			  	column_default as default_value
			 FROM information_schema.columns
			 WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
			 ORDER BY table_schema, table_name, ordinal_position`
	return adapter.QueryCtx(ctx, query)
}
