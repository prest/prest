package controllers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

// TestSelectJoin_RestrictedFields covers reads with _join on a server where
// access.restrict is on (testdata/prest.toml). The select list comes from the
// ACL, so it must qualify the permitted columns of the requested table (an
// unqualified column is ambiguous as soon as a table sharing its name is
// joined) and it must expose columns of the joined table only when that table
// is readable as well.
func TestSelectJoin_RestrictedFields(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	// Join test_group_by_table (read on id, name, age, salary) with test7,
	// which prest.toml does not list and therefore is not readable. Both
	// tables have a "name" column, so the request only succeeds when the
	// select list is qualified, and it must carry no test7 column.
	rows := selectJoinRows(t, base+
		"/prest-test/public/test_group_by_table?_join=inner:test7:test_group_by_table.name:$eq:test7.name",
		token)
	require.Len(t, rows, 1)
	require.Equal(t, "gopher", rows[0]["name"])
	require.NotContains(t, rows[0], "surname", "test7 is not readable, its columns must stay out of the response")

	// Same request joining view_test instead, which is granted read on
	// "player": a readable joined table contributes its permitted columns.
	rows = selectJoinRows(t, base+
		"/prest-test/public/test_group_by_table?_join=inner:view_test:test_group_by_table.name:$eq:view_test.player",
		token)
	require.Len(t, rows, 1)
	require.Equal(t, "gopher", rows[0]["player"])

	// Asking for a column of the unreadable joined table explicitly leaves the
	// request without any permitted column, answered with 400.
	testutils.DoRequestWithHeaders(t, base+
		"/prest-test/public/test_group_by_table?_join=inner:test7:test_group_by_table.name:$eq:test7.name&_select=test7.surname",
		nil, http.MethodGet, http.StatusBadRequest, "SelectJoinRestrictedFields",
		map[string]string{"Authorization": "Bearer " + token})
}

// TestSelectJoin_WildcardFieldsStayOnRequestedTable guards the wildcard grant:
// prest.toml gives every field ("*") on test5, which must not extend to a
// joined table the caller cannot read. A bare "*" in the select list would
// return every column of the join, including the unreadable ones.
func TestSelectJoin_WildcardFieldsStayOnRequestedTable(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")
	const name = "join-acl-wildcard"

	// Seed the row this test reads back, and drop it afterwards so the shared
	// test5 table is left as it was found.
	helpers.DoAuthRequest(t, base+"/prest-test/public/test5",
		map[string]string{"name": name}, http.MethodPost, token,
		http.StatusCreated, "SelectJoinWildcardSeed")
	t.Cleanup(func() {
		helpers.DoAuthRequest(t, base+"/prest-test/public/test5?name=$eq."+name,
			nil, http.MethodDelete, token, http.StatusOK, "SelectJoinWildcardCleanup")
	})

	// Wildcard access on test5 joined to the unreadable test7: the row must
	// hold the test5 columns only.
	rows := selectJoinRows(t, base+
		"/prest-test/public/test5?test5.name=$eq."+name+
		"&_join=left:test7:test5.name:$eq:test7.name",
		token)
	require.Len(t, rows, 1)
	require.Contains(t, rows[0], "celphone", "wildcard access still returns every test5 column")
	require.NotContains(t, rows[0], "surname", "test7 is not readable, its columns must stay out of the response")

	// Requesting a test7 column explicitly is refused even under wildcard
	// access, leaving no permitted column to select.
	testutils.DoRequestWithHeaders(t, base+
		"/prest-test/public/test5?_join=left:test7:test5.name:$eq:test7.name&_select=test7.surname",
		nil, http.MethodGet, http.StatusBadRequest, "SelectJoinWildcardFields",
		map[string]string{"Authorization": "Bearer " + token})
}

func selectJoinRows(t *testing.T, url, token string) []map[string]any {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var rows []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rows))
	return rows
}
