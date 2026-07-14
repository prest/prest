package postgres

import (
	"context"
	"testing"

	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestScriptPermissions_OpenMode(t *testing.T) {
	t.Parallel()

	adapter := New(&config.Prest{QueriesConf: config.QueriesConf{Restrict: false}}).(*postgres)
	require.True(t, adapter.ScriptPermissions(context.Background(), "", "fulltable", "get_all", "read", ""))
}

func TestScriptPermissions_GlobalRule(t *testing.T) {
	t.Parallel()

	adapter := New(&config.Prest{QueriesConf: config.QueriesConf{
		Restrict: true,
		Scripts: []config.ScriptConf{{
			Location:    "fulltable",
			Name:        "get_all",
			Permissions: []string{"read"},
		}},
	}}).(*postgres)

	require.True(t, adapter.ScriptPermissions(context.Background(), "", "fulltable", "get_all", "read", ""))
	require.False(t, adapter.ScriptPermissions(context.Background(), "", "fulltable", "get_all", "write", ""))
	require.False(t, adapter.ScriptPermissions(context.Background(), "", "other", "get_all", "read", ""))
}

func TestScriptPermissions_UserOverride(t *testing.T) {
	t.Parallel()

	adapter := New(&config.Prest{QueriesConf: config.QueriesConf{
		Restrict: true,
		Scripts: []config.ScriptConf{{
			Location:    "fulltable",
			Name:        "get_all",
			Permissions: []string{"read"},
		}},
		Users: []config.QueryUsersConf{{
			Name: "alice",
			Scripts: []config.ScriptConf{{
				Location:    "fulltable",
				Name:        "get_all",
				Permissions: []string{"read", "write"},
			}},
		}},
	}}).(*postgres)

	require.True(t, adapter.ScriptPermissions(context.Background(), "", "fulltable", "get_all", "write", "alice"))
	require.False(t, adapter.ScriptPermissions(context.Background(), "", "fulltable", "get_all", "write", "bob"))
}
