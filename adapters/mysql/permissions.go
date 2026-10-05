package mysql

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/prest/prest/v2/adapters/access"
	"github.com/prest/prest/v2/internal/ident"
)

func (a *Adapter) TablePermissions(database, schema, table, op, userName string) bool {
	return access.TableAllowed(a.cfg, database, schema, table, op, userName)
}

func (a *Adapter) ScriptPermissions(_ context.Context, databaseAlias, location, name, op, userName string) bool {
	return access.ScriptAllowed(a.cfg, databaseAlias, location, name, op, userName)
}

func (a *Adapter) FieldsPermissions(r *http.Request, database, schema, table, op, userName string) ([]string, error) {
	cols, err := columnsByRequest(r)
	if err != nil {
		return nil, fmt.Errorf("error on parse columns from request: %w", err)
	}
	targets, err := joinTargetsByRequest(r, schema, table)
	if err != nil {
		return nil, err
	}
	restrict := a.cfg != nil && a.cfg.AccessConf.Restrict
	if !restrict || op == "delete" {
		if len(cols) > 0 {
			return cols, nil
		}
		return []string{"*"}, nil
	}
	allowed := access.FieldsFor(a.cfg, database, schema, table, op, userName)
	if len(targets) > 0 {
		return joinedFields(a, database, table, targets, allowed, cols, op, userName), nil
	}
	if containsAsterisk(allowed) {
		if len(cols) > 0 {
			return cols, nil
		}
		return []string{"*"}, nil
	}
	fields := intersection(cols, allowed)
	if len(cols) == 0 && len(allowed) > 0 {
		fields = allowed
	}
	return fields, nil
}

type joinTarget struct {
	schema string
	table  string
}

func joinTargetsByRequest(r *http.Request, schema, table string) ([]joinTarget, error) {
	var targets []joinTarget
	for _, clause := range r.URL.Query()["_join"] {
		target, ok := joinTargetByClause(clause, schema)
		if !ok {
			continue
		}
		if target.table == table || slices.ContainsFunc(targets, func(t joinTarget) bool { return t.table == target.table }) {
			return nil, errors.Wrapf(errInvalidJoin, "table %q is joined more than once", target.table)
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func joinTargetByClause(clause, schema string) (joinTarget, bool) {
	joinArgs := strings.Split(clause, ":")
	if len(joinArgs) != 5 {
		return joinTarget{}, false
	}
	parts := strings.Split(joinArgs[1], ".")
	for _, part := range parts {
		if !ident.IsValid(part) {
			return joinTarget{}, false
		}
	}
	switch len(parts) {
	case 1:
		return joinTarget{schema: schema, table: parts[0]}, true
	case 2:
		return joinTarget{schema: parts[0], table: parts[1]}, true
	default:
		return joinTarget{}, false
	}
}

type joinedTable struct {
	table   string
	allowed []string
}

func joinedFields(a *Adapter, database, table string, targets []joinTarget, allowedFields, cols []string, op, userName string) []string {
	tables := []joinedTable{{table: table, allowed: allowedFields}}
	for _, target := range targets {
		var joinAllowed []string
		if a.TablePermissions(database, target.schema, target.table, op, userName) {
			joinAllowed = access.FieldsFor(a.cfg, database, target.schema, target.table, op, userName)
		}
		tables = append(tables, joinedTable{table: target.table, allowed: joinAllowed})
	}
	if len(cols) == 0 {
		return allJoinedFields(tables)
	}
	var fields []string
	for _, col := range cols {
		if col == "*" {
			fields = append(fields, allJoinedFields(tables)...)
			continue
		}
		if field, permitted := permittedJoinedField(col, tables); permitted {
			fields = append(fields, field)
		}
	}
	return fields
}

func allJoinedFields(tables []joinedTable) []string {
	var fields []string
	for _, t := range tables {
		fields = append(fields, tableFields(t.table, t.allowed)...)
	}
	return fields
}

func tableFields(table string, allowed []string) []string {
	if len(allowed) == 0 {
		return nil
	}
	if containsAsterisk(allowed) {
		return []string{table + ".*"}
	}
	fields := make([]string, 0, len(allowed))
	for _, field := range allowed {
		fields = append(fields, qualifyField(table, field))
	}
	return fields
}

func permittedJoinedField(col string, tables []joinedTable) (string, bool) {
	queried := tables[0]
	if checkField(col, queried.allowed) != "" {
		return qualifyField(queried.table, col), true
	}
	if prefix, name, qualified := strings.Cut(col, "."); qualified && !strings.Contains(name, ".") {
		for _, t := range tables {
			if t.table == prefix {
				return col, containsAsterisk(t.allowed) || checkField(name, t.allowed) != ""
			}
		}
		return "", false
	}
	return qualifyField(queried.table, col), containsAsterisk(queried.allowed)
}

func qualifyField(table, field string) string {
	if strings.Contains(field, ".") || !ident.IsValid(field) || !ident.IsValid(table) {
		return field
	}
	return table + "." + field
}

func containsAsterisk(arr []string) bool {
	return slices.Contains(arr, "*")
}

func intersection(set, other []string) []string {
	var out []string
	for _, field := range set {
		if p := checkField(field, other); p != "" {
			out = append(out, p)
		}
	}
	return out
}

var groupFieldRegex = regexp.MustCompile("[`\"](.+?)[`\"]")

func checkField(col string, fields []string) string {
	fieldName := groupFieldRegex.FindStringSubmatch(col)
	for _, f := range fields {
		if len(fieldName) == 2 && fieldName[1] == f {
			return col
		}
		if col == f {
			return col
		}
	}
	return ""
}

func columnsByRequest(r *http.Request) ([]string, error) {
	var columns []string
	for _, j := range r.URL.Query()["_select"] {
		for _, arg := range strings.Split(j, ",") {
			field := strings.TrimSpace(arg)
			if field != "" {
				columns = append(columns, field)
			}
		}
	}
	if r.URL.Query().Get("_groupby") == "" {
		return columns, nil
	}
	for i, col := range columns {
		if strings.Contains(col, ":") {
			gf, err := normalizeGroupFunction(col)
			if err != nil {
				return nil, err
			}
			columns[i] = gf
		}
	}
	return columns, nil
}
