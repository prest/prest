package postgres

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/prest/prest/v2/pkg/config"
)

// TablePermissions get tables permissions based in prest configuration
func (adapter *postgres) TablePermissions(database, schema, table, op, userName string) (access bool) {
	restrict := adapter.cfg.AccessConf.Restrict
	if !restrict {
		return true
	}

	for _, ignoreT := range adapter.cfg.AccessConf.IgnoreTable {
		if ignoreT == table {
			return true
		}
	}

	if t, ok := matchTableConf(adapter.cfg.AccessConf.Tables, database, schema, table); ok {
		access = slices.Contains(t.Permissions, op)
	} else {
		access = false
	}

	if userName == "" {
		return access
	}

	users := adapter.cfg.AccessConf.Users
	for _, u := range users {
		if u.Name != userName {
			continue
		}
		if t, ok := matchTableConf(u.Tables, database, schema, table); ok {
			return slices.Contains(t.Permissions, op)
		}
	}
	return access
}

func matchTableConf(tables []config.TablesConf, database, schema, table string) (config.TablesConf, bool) {
	var tableOnly, schemaTable, full *config.TablesConf
	for i := range tables {
		t := &tables[i]
		if t.Name != table {
			continue
		}
		switch {
		case t.Database == database && t.Schema == schema:
			full = t
		case t.Database == "" && t.Schema == schema:
			schemaTable = t
		case t.Database == "" && t.Schema == "":
			tableOnly = t
		}
	}
	if full != nil {
		return *full, true
	}
	if schemaTable != nil {
		return *schemaTable, true
	}
	if tableOnly != nil {
		return *tableOnly, true
	}
	return config.TablesConf{}, false
}

// fieldsByPermission returns a list of fields that a user is allowed to access
// for a given table and operation based on the configuration.
func (adapter *postgres) fieldsByPermission(database, schema, table, operation, userName string) (fields []string) {
	fields = []string{"*"}

	if t, ok := matchTableConf(adapter.cfg.AccessConf.Tables, database, schema, table); ok {
		for _, perm := range t.Permissions {
			if perm == operation {
				fields = t.Fields
			}
		}
	}

	if userName == "" {
		return
	}

	users := adapter.cfg.AccessConf.Users
	for _, u := range users {
		if u.Name != userName {
			continue
		}
		if t, ok := matchTableConf(u.Tables, database, schema, table); ok &&
			slices.Contains(t.Permissions, operation) {
			fields = t.Fields
		}
	}

	return
}

func containsAsterisk(arr []string) bool {
	for _, e := range arr {
		if e == "*" {
			return true
		}
	}
	return false
}

func intersection(set, other []string) (intersection []string) {
	for _, field := range set {
		pField := checkField(field, other)
		if pField != "" {
			intersection = append(intersection, pField)
		}
	}
	return
}

// FieldsPermissions get fields permissions based in prest configuration
func (adapter *postgres) FieldsPermissions(r *http.Request, database, schema, table, op, userName string) (fields []string, err error) {
	cols, err := columnsByRequest(r)
	if err != nil {
		err = fmt.Errorf("error on parse columns from request: %s", err)
		return
	}
	restrict := adapter.cfg.AccessConf.Restrict
	if !restrict || op == "delete" {
		if len(cols) > 0 {
			fields = cols
			return
		}
		fields = []string{"*"}
		return
	}
	allowedFields := adapter.fieldsByPermission(database, schema, table, op, userName)
	if containsAsterisk(allowedFields) {
		fields = []string{"*"}
		if len(cols) > 0 {
			fields = cols
		}
		return
	}
	fields = intersection(cols, allowedFields)
	if len(cols) == 0 && len(allowedFields) > 0 {
		fields = allowedFields
	}
	return
}

// groupRegex matches aggregate function calls like COUNT(field) or MAX(col)
var groupRegex = regexp.MustCompile(`\"(.+?)\"`)

func checkField(col string, fields []string) (p string) {
	// regex get field from func group
	fieldName := groupRegex.FindStringSubmatch(col)
	for _, f := range fields {
		if len(fieldName) == 2 && fieldName[1] == f {
			p = col
			return
		}
		if col == f {
			p = col
			return
		}
	}
	return
}

func normalizeAll(cols []string) (pCols []string, err error) {
	for _, col := range cols {
		var gf string
		gf, err = normalizeColumn(col)
		if err != nil {
			return
		}
		pCols = append(pCols, gf)
	}
	return
}

func normalizeColumn(col string) (gf string, err error) {
	if strings.Contains(col, ":") {
		gf, err = NormalizeGroupFunction(col)
		return
	}
	gf = col
	return
}

// columnsByRequest extract columns and return as array of strings
func columnsByRequest(r *http.Request) (columns []string, err error) {
	queries := r.URL.Query()
	columnsArr := queries["_select"]
	for _, j := range columnsArr {
		cArgs := strings.Split(j, ",")
		for _, arg := range cArgs {
			field := strings.TrimSpace(arg)
			if field != "" {
				columns = append(columns, field)
			}
		}
	}
	if queries.Get("_groupby") != "" {
		columns, err = normalizeAll(columns)
		if err != nil {
			return
		}
	}
	return
}
