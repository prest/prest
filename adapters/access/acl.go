package access

import (
	"slices"

	"github.com/prest/prest/v2/config"
)

// TableAllowed reports whether op is permitted on database.schema.table.
// restrict off allows every table. IgnoreTable entries are always allowed.
// A matching user rule replaces the table rule for that user.
func TableAllowed(cfg *config.Prest, database, schema, table, op, userName string) bool {
	if cfg == nil || !cfg.AccessConf.Restrict {
		return true
	}
	for _, ignoreT := range cfg.AccessConf.IgnoreTable {
		if ignoreT == table {
			return true
		}
	}

	access := false
	if t, ok := matchTableConf(cfg.AccessConf.Tables, database, schema, table); ok {
		access = slices.Contains(t.Permissions, op)
	}
	if userName == "" {
		return access
	}
	for _, u := range cfg.AccessConf.Users {
		if u.Name != userName {
			continue
		}
		if t, ok := matchTableConf(u.Tables, database, schema, table); ok {
			return slices.Contains(t.Permissions, op)
		}
	}
	return access
}

// FieldsFor returns the configured field list for operation.
// No matching rule yields ["*"]. A user rule replaces the table rule when
// that user is allowed the operation.
func FieldsFor(cfg *config.Prest, database, schema, table, operation, userName string) []string {
	fields := []string{"*"}
	if cfg == nil {
		return fields
	}
	if t, ok := matchTableConf(cfg.AccessConf.Tables, database, schema, table); ok {
		for _, perm := range t.Permissions {
			if perm == operation {
				fields = t.Fields
			}
		}
	}
	if userName == "" {
		return fields
	}
	for _, u := range cfg.AccessConf.Users {
		if u.Name != userName {
			continue
		}
		if t, ok := matchTableConf(u.Tables, database, schema, table); ok &&
			slices.Contains(t.Permissions, operation) {
			fields = t.Fields
		}
	}
	return fields
}

// ScriptAllowed reports whether op is permitted on a stored script.
// restrict off allows every script. A matching user rule replaces the script rule.
func ScriptAllowed(cfg *config.Prest, databaseAlias, location, name, op, userName string) bool {
	if cfg == nil || !cfg.QueriesConf.Restrict {
		return true
	}
	access := false
	if s, ok := matchScriptConf(cfg.QueriesConf.Scripts, databaseAlias, location, name); ok {
		access = slices.Contains(s.Permissions, op)
	}
	if userName == "" {
		return access
	}
	for _, u := range cfg.QueriesConf.Users {
		if u.Name != userName {
			continue
		}
		if s, ok := matchScriptConf(u.Scripts, databaseAlias, location, name); ok {
			return slices.Contains(s.Permissions, op)
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

func matchScriptConf(scripts []config.ScriptConf, databaseAlias, location, name string) (config.ScriptConf, bool) {
	var locationOnly, full *config.ScriptConf
	for i := range scripts {
		s := &scripts[i]
		if !scriptNameMatches(s.Name, name) {
			continue
		}
		if !scriptLocationMatches(s.Location, location) {
			continue
		}
		switch {
		case s.Database == databaseAlias:
			full = s
		case s.Database == "":
			locationOnly = s
		}
	}
	if full != nil {
		return *full, true
	}
	if locationOnly != nil {
		return *locationOnly, true
	}
	return config.ScriptConf{}, false
}

func scriptNameMatches(rule, name string) bool {
	if rule == "" || rule == "*" {
		return true
	}
	return rule == name
}

func scriptLocationMatches(rule, location string) bool {
	if rule == "" || rule == "*" {
		return true
	}
	return rule == location
}
