package postgres

import "net/http"

// ChkInvalidIdentifierExported exposes chkInvalidIdentifier for integration tests.
func ChkInvalidIdentifierExported(identifier ...string) bool {
	return chkInvalidIdentifier(identifier...)
}

// ColumnsByRequestExported exposes columnsByRequest for integration tests.
func ColumnsByRequestExported(r *http.Request) ([]string, error) {
	return columnsByRequest(r)
}

// FieldsByPermissionExported exposes fieldsByPermission for integration tests.
func FieldsByPermissionExported(table, operation, userName string) []string {
	return fieldsByPermission(table, operation, userName)
}
