package reqcols

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExtract lists the columns each request position names, exactly where
// the SQL builders read them, so the controller can check each one against
// the field ACL (GHSA-48r9-g6f9-x3gm).
func TestExtract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"plain filter", "ssn=$eq.1", []string{"ssn"}},
		{"qualified filter", "employee_secret.ssn=$like.1%25", []string{"employee_secret.ssn"}},
		{"jsonb filter", "meta->>k:jsonb=$eq.v", []string{"meta"}},
		{"tsquery filter", "doc:tsquery=x", []string{"doc"}},
		{"vecdist filter", "emb:vecdist=l2:lt:[1]:0.5", []string{"emb"}},
		{"or groups", "_or=a=$eq.1 OR b.c=$eq.'x OR y'", []string{"a", "b.c"}},
		{"or legacy separator", "_or=a=$eq.1||b=$eq.2", []string{"a", "b"}},
		{"count star", "_count=*", nil},
		{"count columns", "_count=ssn,id", []string{"ssn", "id"}},
		{"groupby columns", "_groupby=dept,ssn", []string{"dept", "ssn"}},
		{"groupby having", "_groupby=dept->>having:sum:salary:$gt:1", []string{"dept", "salary"}},
		{"groupby having star", "_groupby=dept->>having:count:*:$gt:1", []string{"dept"}},
		{"groupby function", "_groupby=lower(ssn)", []string{"ssn"}},
		{"groupby function literal", "_groupby=upper('x' || ssn)", []string{"ssn"}},
		// groupByParts splits on "," before it sees a function, so a
		// multi-argument call never reaches SQL and names no column.
		{"groupby multi-arg function dropped", "_groupby=date_trunc('day', created_at)", nil},
		{"groupby extract", "_groupby=extract(year FROM ts)", []string{"ts"}},
		{"order", "_order=-ssn,dept", []string{"ssn", "dept"}},
		{"reserved params ignored", "_page=1&_select=x&_join=inner:t:a.b:$eq:t.c&_distinct=true", nil},
		{"invalid identifier skipped", "bad-col=$eq.1", nil},
		{"duplicates removed", "a=$eq.1&_order=a", []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := url.ParseQuery(tc.query)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.want, Extract(q))
		})
	}
}

// TestSplitTopLevelOrGroup keeps quoted separators inside their group, as the
// builders' splitter does.
func TestSplitTopLevelOrGroup(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"a=$eq.1", "b=$eq.'x OR y'"}, SplitTopLevelOrGroup("a=$eq.1 OR b=$eq.'x OR y'"))
	require.Equal(t, []string{`a=$eq."p||q"`, "b=$eq.2"}, SplitTopLevelOrGroup(`a=$eq."p||q"||b=$eq.2`))
	require.Equal(t, []string{"a=$eq.'it''s'"}, SplitTopLevelOrGroup("a=$eq.'it''s'"))
	require.Empty(t, SplitTopLevelOrGroup("  "))
}
