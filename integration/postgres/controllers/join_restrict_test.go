package controllers_test

import (
	"net/http"
	"sort"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

// joinRow is a decoded response row of the department/employee join. Every
// assertion below is about which keys the row carries, so the response is
// decoded into a map rather than substring-matched.
type joinRow map[string]interface{}

// departmentEmployeeJoin joins department to the employee it belongs to.
// employee is readable in testdata/prest.toml, so its columns must reach the
// response even with access.restrict = true.
const departmentEmployeeJoin = "/prest-test/public/department" +
	"?_join=inner:employee:department.emp_id:$eq:employee.id" +
	"&_order=d_id"

// departmentSecretJoin joins department to employee_secret, which
// testdata/prest.toml grants "write" only. The join is legal SQL, but no
// column of employee_secret may appear in the response.
const departmentSecretJoin = "/prest-test/public/employee_secret" +
	"?_join=inner:department:employee_secret.emp_id:$eq:department.emp_id"

func keysOf(t *testing.T, rows []joinRow) map[string]bool {
	t.Helper()
	require.NotEmpty(t, rows, "join returned no row; the fixtures in testdata/schema.sql should yield two")
	keys := map[string]bool{}
	for _, row := range rows {
		for k := range row {
			keys[k] = true
		}
	}
	return keys
}

// TestJoinRestricted_ReturnsJoinedTableColumns is issue #364: with
// access.restrict = true the select list was built from the queried table's
// permitted fields only, so the joined table contributed no column even though
// the config granted read access to it. The auth server runs
// testdata/prest.toml unmodified, i.e. with restrict = true.
func TestJoinRestricted_ReturnsJoinedTableColumns(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	// GET department joined to employee. Expected: the department columns the
	// ACL permits (d_id, dept, emp_id) *and* employee's (id, name).
	var rows []joinRow
	helpers.DoAuthRequestJSON(
		t, base+departmentEmployeeJoin,
		nil, http.MethodGet, token, http.StatusOK, "JoinRestricted", &rows)

	keys := keysOf(t, rows)
	for _, col := range []string{"d_id", "dept", "emp_id", "id", "name"} {
		require.True(t, keys[col], "column %q missing from restricted join response: %v", col, rows)
	}

	// The join must still return the joined rows themselves, not just the shape.
	require.Equal(t, "Computer", rows[0]["dept"])
	require.Equal(t, "gopher", rows[0]["name"])
}

// TestJoinUnrestricted_ReturnsJoinedTableColumns is the contrast the fix aims
// at: the same join on the server that runs with restrict = false. Restricted
// and unrestricted responses must agree on which joined columns come back.
func TestJoinUnrestricted_ReturnsJoinedTableColumns(t *testing.T) {
	base := helpers.ServerURL(t)

	var rows []joinRow
	testutils.DoRequestJSON(
		t, base+departmentEmployeeJoin,
		nil, http.MethodGet, http.StatusOK, "JoinUnrestricted", &rows)

	keys := keysOf(t, rows)
	for _, col := range []string{"d_id", "dept", "emp_id", "id", "name"} {
		require.True(t, keys[col], "column %q missing from unrestricted join response: %v", col, rows)
	}
}

// TestJoinRestricted_DoesNotLeakUnreadableJoinedTable guards the other half of
// the fix: pulling a table into the query with _join must not hand out columns
// the ACL never granted read access to. employee_secret is write-only in
// testdata/prest.toml, so ssn must never appear.
func TestJoinRestricted_DoesNotLeakUnreadableJoinedTable(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	var testCases = []struct {
		description string
		url         string
		status      int
		wantKeys    []string
		denyKeys    []string
	}{
		{
			// department joined to the unreadable employee_secret: the
			// department columns come back, none of employee_secret's do.
			"a joined table without read permission contributes no column",
			"/prest-test/public/department" +
				"?_join=inner:employee_secret:department.emp_id:$eq:employee_secret.emp_id" +
				"&_order=d_id",
			http.StatusOK,
			[]string{"d_id", "dept", "emp_id"},
			[]string{"ssn"},
		},
		{
			// Asking for the unreadable column explicitly must not promote it:
			// dept survives, employee_secret.ssn is dropped.
			"_select of an unreadable joined column is dropped, the permitted one survives",
			"/prest-test/public/department" +
				"?_join=inner:employee_secret:department.emp_id:$eq:employee_secret.emp_id" +
				"&_select=dept,employee_secret.ssn&_order=d_id",
			http.StatusOK,
			[]string{"dept"},
			[]string{"ssn"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			var rows []joinRow
			helpers.DoAuthRequestJSON(
				t, base+tc.url,
				nil, http.MethodGet, token, tc.status, "JoinRestrictedLeak", &rows)

			keys := keysOf(t, rows)
			for _, col := range tc.wantKeys {
				require.True(t, keys[col], "column %q missing from response: %v", col, rows)
			}
			for _, col := range tc.denyKeys {
				require.False(t, keys[col], "unreadable column %q leaked into response: %v", col, rows)
			}
		})
	}
}

// TestJoinRestricted_SelectOnlyUnreadableColumnIsRejected: when _select names
// nothing the caller may read, the request has no permitted field left and
// must fail rather than fall back to a broader select list.
func TestJoinRestricted_SelectOnlyUnreadableColumnIsRejected(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	// _select asks for employee_secret.ssn alone, which the ACL forbids.
	// Expected: 400 with the "no permission" message, and no row at all.
	helpers.DoAuthRequest(
		t, base+"/prest-test/public/department"+
			"?_join=inner:employee_secret:department.emp_id:$eq:employee_secret.emp_id"+
			"&_select=employee_secret.ssn",
		nil, http.MethodGet, token, http.StatusBadRequest, "JoinRestrictedSelect",
		"you don't have permission for this action")
}

// TestJoinRestricted_SelectQualifiedColumns pins the select syntax the fix
// introduces: a join qualifies columns with their table, so a caller can name
// either side explicitly and gets exactly those columns back.
func TestJoinRestricted_SelectQualifiedColumns(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	// _select=dept,employee.name: one bare column of the queried table and one
	// qualified column of the joined table. Expected: those two keys only.
	var rows []joinRow
	helpers.DoAuthRequestJSON(
		t, base+"/prest-test/public/department"+
			"?_join=inner:employee:department.emp_id:$eq:employee.id"+
			"&_select=dept,employee.name&_order=d_id",
		nil, http.MethodGet, token, http.StatusOK, "JoinRestrictedSelect", &rows)

	require.Equal(t, []string{"dept", "name"}, sortedKeys(keysOf(t, rows)))
	require.Equal(t, "Computer", rows[0]["dept"])
	require.Equal(t, "gopher", rows[0]["name"])
}

// TestJoinRestricted_QueriedTableIsUnreadable: reversing the join does not
// widen anything. employee_secret has no read permission, so querying it
// directly is rejected by the access control middleware, join or not.
func TestJoinRestricted_QueriedTableIsUnreadable(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	helpers.DoAuthRequest(
		t, base+departmentSecretJoin,
		nil, http.MethodGet, token, http.StatusUnauthorized, "JoinRestrictedReverse")
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
