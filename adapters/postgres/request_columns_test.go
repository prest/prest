package postgres

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prest/prest/v2/internal/ident"
	"github.com/stretchr/testify/require"
)

// TestRequestColumns adds the Postgres-only _korder column and the _join
// targets to the shared extraction.
func TestRequestColumns(t *testing.T) {
	t.Parallel()

	a := testAdapter()
	r := httptest.NewRequest(http.MethodGet, "/db/public/t?dept=$eq.x&_order=-id"+
		"&_korder=embedding:l2:[1,2]"+
		"&_join=inner:employee_secret:t.emp_id:$eq:employee_secret.emp_id"+
		"&_join=left:other.badge:t.id:$eq:badge.emp_id", nil)
	got, err := a.RequestColumns(r)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"dept", "id", "embedding"}, got.Columns)
	require.Equal(t, []string{"employee_secret", "badge"}, got.JoinTables)

	empty, err := a.RequestColumns(httptest.NewRequest(http.MethodGet, "/db/public/t", nil))
	require.NoError(t, err)
	require.Empty(t, empty.Columns)
	require.Empty(t, empty.JoinTables)
}

// TestRequestColumns_MatchesBuilders guards against drift: every column the
// lister reports must appear in the SQL the builders produce for the same
// request, so the ACL check covers exactly what reaches the database.
func TestRequestColumns_MatchesBuilders(t *testing.T) {
	t.Parallel()

	a := testAdapter()
	for _, query := range []string{
		"ssn=$eq.1&employee_secret.ssn=$like.1%25",
		"meta->>k:jsonb=$eq.v",
		"_or=a=$eq.1 OR b.c=$eq.2",
		"_count=ssn,id",
		"_groupby=dept,ssn",
		"_groupby=dept->>having:sum:salary:$gt:1",
		"_groupby=lower(ssn)",
		"_order=-ssn,dept",
		"_korder=embedding:l2:[1,2]",
	} {
		t.Run(query, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/db/public/t?"+strings.ReplaceAll(query, " ", "%20"), nil)
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
				quoted, _ := ident.Quote(col)
				require.True(t, strings.Contains(sql, quoted) || strings.Contains(sql, "("+col+")"),
					"column %q listed but not in builder SQL %q", col, sql)
			}
		})
	}
}

// TestCountByRequest_AcceptsPermittedForms: Select hands CountByRequest the
// FieldsPermissions output as _select (GHSA-48r9-g6f9-x3gm), so every form
// that list can hold (bare, table.col, table.*, an aggregate under _groupby)
// must still build a count query, and nothing outside it may appear.
func TestCountByRequest_AcceptsPermittedForms(t *testing.T) {
	t.Parallel()

	a := testAdapter(joinPermissionTestConf())
	for _, raw := range []string{
		"/public/department?_join=inner:employee_secret:department.emp_id:$eq:employee_secret.emp_id&_select=dept,employee_secret.ssn&_count=*&_groupby=dept",
		"/public/department?_join=inner:ignored_table:department.d_id:$eq:ignored_table.d_id&_select=dept,ignored_table.*&_count=*",
		"/public/employee?_select=name,sum:id&_count=*&_groupby=name",
	} {
		t.Run(raw, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, raw, nil)
			table := strings.Split(strings.TrimPrefix(r.URL.Path, "/public/"), "?")[0]
			cols, err := a.FieldsPermissions(r, "prest-test", "public", table, "read", "")
			require.NoError(t, err)
			require.NotEmpty(t, cols)

			q := r.URL.Query()
			q.Set("_select", strings.Join(cols, ","))
			cr := r.Clone(r.Context())
			cr.URL.RawQuery = q.Encode()
			count, err := a.CountByRequest(cr)
			require.NoError(t, err, "cols %v", cols)
			require.NotContains(t, count, "ssn")
		})
	}
}
