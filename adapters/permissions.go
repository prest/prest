package adapters

import "net/http"

// PermissionsChecker validates table and field access for users.
type PermissionsChecker interface {
	TablePermissions(database, schema, table, op, userName string) bool
	FieldsPermissions(r *http.Request, database, schema, table, op, userName string) (fields []string, err error)
}
