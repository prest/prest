package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBindValueJSONAsString(t *testing.T) {
	// Objects and arrays bind as strings so interpolateParams does not send
	// them as _binary literals, which JSON columns reject (error 3144).
	got, err := bindValue(map[string]interface{}{"kind": "apparel"})
	require.NoError(t, err)
	require.Equal(t, `{"kind":"apparel"}`, got)

	got, err = bindValue([]interface{}{1, "a"})
	require.NoError(t, err)
	require.Equal(t, `[1,"a"]`, got)

	got, err = bindValue([2]int{1, 2})
	require.NoError(t, err)
	require.Equal(t, `[1,2]`, got)

	got, err = bindValue([]byte("raw"))
	require.NoError(t, err)
	require.Equal(t, []byte("raw"), got)

	got, err = bindValue(nil)
	require.NoError(t, err)
	require.Nil(t, got)

	_, err = bindValue(map[string]interface{}{"bad": make(chan int)})
	require.Error(t, err)
}

// Aggregates without an explicit alias get the lowercase function name, so the
// JSON key matches Postgres ("sum") instead of "SUM(`n`)".
func TestNormalizeGroupFunctionDefaultAlias(t *testing.T) {
	t.Parallel()
	got, err := normalizeGroupFunction("sum:n")
	require.NoError(t, err)
	require.Equal(t, "SUM(`n`) AS `sum`", got)

	got, err = normalizeGroupFunction("avg:*")
	require.NoError(t, err)
	require.Equal(t, "AVG(*) AS `avg`", got)

	got, err = normalizeGroupFunction("sum:n:total")
	require.NoError(t, err)
	require.Equal(t, "SUM(`n`) AS `total`", got)

	a := New(testCfg()).(*Adapter)
	sel, err := a.SelectFields([]string{"status", "max:age"})
	require.NoError(t, err)
	require.Equal(t, "SELECT `status`,MAX(`age`) AS `max` FROM", sel)
}

// HAVING must use the bare aggregate expression, never the aliased form.
func TestGroupByHavingUsesBareAggregate(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)
	require.Equal(t, "GROUP BY `status` HAVING AVG(`age`) > 18",
		a.GroupByClause(req("/t?_groupby=status->>having:avg:age:$gt:18")))
	clause, vals := a.GroupByClauseValues(req("/t?_groupby=status->>having:sum:n:$gte:5"), 1)
	require.Equal(t, "GROUP BY `status` HAVING SUM(`n`) >= ?", clause)
	require.Equal(t, []any{"5"}, vals)
}
