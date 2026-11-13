package mysql

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRequestColumns lists the shared positions and the _join targets;
// MySQL has no _korder, so it adds nothing for it.
func TestRequestColumns(t *testing.T) {
	t.Parallel()

	a := New(testCfg()).(*Adapter)
	r := httptest.NewRequest(http.MethodGet, "/shop/shop/items?name=$eq.x&_order=-id"+
		"&_join=inner:tags:items.id:$eq:tags.id", nil)
	got, err := a.RequestColumns(r)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"name", "id"}, got.Columns)
	require.Equal(t, []string{"tags"}, got.JoinTables)
}

// TestRequestColumns_MatchesBuilders guards against drift between the lister
// and the MySQL SQL builders.
func TestRequestColumns_MatchesBuilders(t *testing.T) {
	t.Parallel()

	a := New(testCfg()).(*Adapter)
	for _, query := range []string{
		"name=$eq.1&items.qty=$gt.1",
		"meta->>k:jsonb=$eq.v",
		"_or=a=$eq.1 OR b.c=$eq.2",
		"_count=name,id",
		"_groupby=name,qty",
		"_groupby=name->>having:sum:qty:$gt:1",
		"_groupby=lower(name)",
		"_order=-qty,name",
	} {
		t.Run(query, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/shop/shop/items?"+strings.ReplaceAll(query, " ", "%20"), nil)
			refs, err := a.RequestColumns(r)
			require.NoError(t, err)
			require.NotEmpty(t, refs.Columns)

			where, _, err := a.WhereByRequest(r, 1)
			require.NoError(t, err)
			count, err := a.CountByRequest(r)
			require.NoError(t, err)
			order, err := a.OrderByRequest(r)
			require.NoError(t, err)
			sql := strings.Join([]string{where, count, a.GroupByClause(r), order}, " ")

			for _, col := range refs.Columns {
				quoted, err := quoteIdent(col)
				require.NoError(t, err)
				require.True(t, strings.Contains(sql, quoted) || strings.Contains(sql, "("+col+")"),
					"column %q listed but not in builder SQL %q", col, sql)
			}
		})
	}
}
