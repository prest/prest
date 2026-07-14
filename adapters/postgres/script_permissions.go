package postgres

import (
	"context"
	"slices"

	"github.com/prest/prest/v2/config"
)

// ScriptPermissions checks whether a user may execute a stored query script.
// ctx is reserved for future DB-backed permission checks; the current implementation is config-only.
func (adapter *postgres) ScriptPermissions(_ context.Context, databaseAlias, location, name, op, userName string) bool {
	qc := adapter.cfg.QueriesConf
	if !qc.Restrict {
		return true
	}

	access := false
	if s, ok := matchScriptConf(qc.Scripts, databaseAlias, location, name); ok {
		access = slices.Contains(s.Permissions, op)
	}

	if userName == "" {
		return access
	}

	for _, u := range qc.Users {
		if u.Name != userName {
			continue
		}
		if s, ok := matchScriptConf(u.Scripts, databaseAlias, location, name); ok {
			return slices.Contains(s.Permissions, op)
		}
	}
	return access
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
