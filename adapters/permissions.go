package adapters

import "net/http"

// PermissionsChecker validates table and field access for users.
type PermissionsChecker interface {
	TablePermissions(table string, op string, userName string) bool
	FieldsPermissions(r *http.Request, table string, op string, userName string) (fields []string, err error)
}
