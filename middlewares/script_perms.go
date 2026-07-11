package middlewares

import "github.com/prest/prest/v2/adapters"

type allowAllScriptPerms struct{}

func (allowAllScriptPerms) ScriptPermissions(_, _, _, _, _ string) bool { return true }

// ScriptPermsFromAdapter returns ScriptPermissionsChecker from the adapter or allow-all when absent.
func ScriptPermsFromAdapter(a adapters.Adapter) adapters.ScriptPermissionsChecker {
	if perms, ok := a.(adapters.ScriptPermissionsChecker); ok {
		return perms
	}
	return allowAllScriptPerms{}
}
