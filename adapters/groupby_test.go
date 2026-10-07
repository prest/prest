package adapters_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/stretchr/testify/require"
)

// groupByStringBuilder implements RequestQueryBuilder and not GroupByBinder.
type groupByStringBuilder struct{}

func (groupByStringBuilder) WhereByRequest(*http.Request, int) (string, []interface{}, error) {
	return "", nil, nil
}
func (groupByStringBuilder) DistinctClause(*http.Request) (string, error) { return "", nil }
func (groupByStringBuilder) OrderByRequest(*http.Request) (string, error) { return "", nil }
func (groupByStringBuilder) PaginateIfPossible(*http.Request) (string, error) {
	return "", nil
}
func (groupByStringBuilder) JoinByRequest(*http.Request) ([]string, error) { return nil, nil }
func (groupByStringBuilder) GroupByClause(*http.Request) string            { return `GROUP BY "name"` }
func (groupByStringBuilder) TimeBucketClause(*http.Request) (string, error) {
	return "", nil
}
func (groupByStringBuilder) CountByRequest(*http.Request) (string, error) { return "", nil }
func (groupByStringBuilder) ReturningByRequest(*http.Request) (string, error) {
	return "", nil
}
func (groupByStringBuilder) SetByRequest(*http.Request, int) (string, []interface{}, error) {
	return "", nil, nil
}
func (groupByStringBuilder) ParseInsertRequest(*http.Request) (string, string, []interface{}, error) {
	return "", "", nil, nil
}
func (groupByStringBuilder) ParseBatchInsertRequest(*http.Request) (string, string, []interface{}, error) {
	return "", "", nil, nil
}

// groupByBinderBuilder implements GroupByBinder. GroupByClause is a different
// string so a fallback through that method cannot satisfy the assertion.
type groupByBinderBuilder struct {
	groupByStringBuilder
}

func (groupByBinderBuilder) GroupByClause(*http.Request) string { return "unused" }

func (groupByBinderBuilder) GroupByClauseValues(*http.Request, int) (string, []any) {
	return "GROUP BY x", []any{1}
}

func TestGroupByFromRequest(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)

	clause, values := adapters.GroupByFromRequest(groupByStringBuilder{}, req, 1)
	require.Equal(t, `GROUP BY "name"`, clause)
	require.Nil(t, values)

	clause, values = adapters.GroupByFromRequest(groupByBinderBuilder{}, req, 1)
	require.Equal(t, "GROUP BY x", clause)
	require.Equal(t, []any{1}, values)
}
