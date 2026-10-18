package mysql

import (
	"testing"

	"github.com/prest/prest/v2/adapters/access"
	"github.com/stretchr/testify/require"
)

// Control-plane tables must be rejected as the queried table or as a join
// target in both restrict modes (GHSA-2x9q-r7j9-g85f).
func TestFieldsPermissionsRejectsReservedTables(t *testing.T) {
	tests := []struct {
		name     string
		queries  string // overrides cfg.QueriesConf.Table when set
		table    string
		query    string
		reserved bool
	}{
		{name: "queried table reserved", table: "prest_queries", query: "/t", reserved: true},
		{name: "unqualified join target", table: "items", query: "/t?_join=inner:prest_queries:items.id:$eq:prest_queries.id", reserved: true},
		{name: "qualified join target", table: "items", query: "/t?_join=inner:shop.prest_users:items.id:$eq:prest_users.id", reserved: true},
		{name: "other db join target", table: "items", query: "/t?_join=inner:other.prest_queries:items.id:$eq:prest_queries.id", reserved: true},
		{name: "upper-case join target", table: "items", query: "/t?_join=inner:PREST_QUERIES:items.id:$eq:PREST_QUERIES.id", reserved: true},
		{name: "configured custom name", queries: "stored_sql", table: "items", query: "/t?_join=inner:stored_sql:items.id:$eq:stored_sql.id", reserved: true},
		{name: "ordinary join target", table: "items", query: "/t?_join=inner:orders:items.id:$eq:orders.id", reserved: false},
	}
	for _, tc := range tests {
		for _, restrict := range []bool{false, true} {
			name := tc.name + "/restrict=false"
			if restrict {
				name = tc.name + "/restrict=true"
			}
			t.Run(name, func(t *testing.T) {
				cfg := testCfg()
				cfg.AccessConf.Restrict = restrict
				if tc.queries != "" {
					cfg.QueriesConf.Table = tc.queries
				}
				a := New(cfg).(*Adapter)
				_, err := a.FieldsPermissions(req(tc.query), "shop", "shop", tc.table, "read", "")
				if tc.reserved {
					require.ErrorIs(t, err, access.ErrReservedTable)
					return
				}
				require.NotErrorIs(t, err, access.ErrReservedTable)
			})
		}
	}
}
