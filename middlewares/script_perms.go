package middlewares

import (
	"context"

	"github.com/prest/prest/v2/adapters"
)

type denyAllScriptPerms struct{}

func (denyAllScriptPerms) ScriptPermissions(_ context.Context, _, _, _, _, _ string) bool {
	return false
}

// ScriptPermsFromAdapter returns ScriptPermissionsChecker from the adapter or deny-all when absent.
func ScriptPermsFromAdapter(a adapters.Adapter) adapters.ScriptPermissionsChecker {
	if perms, ok := a.(adapters.ScriptPermissionsChecker); ok {
		return perms
	}
	return denyAllScriptPerms{}
}
