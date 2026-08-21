package controllers_test

import (
	"net/http"
	"net/url"
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

// TestJoinRestricted_RepeatedJoin covers a request that repeats _join: every
// clause has to reach both the FROM clause and the permitted-column list, so
// the readable joined table contributes its columns while the unreadable one
// still contributes none.
func TestJoinRestricted_RepeatedJoin(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	// department joined to employee and employee_badge (both readable) and to
	// employee_secret (write-only). Expected: the columns of the first three,
	// none of employee_secret's. badge only appears if the *second* clause
	// reached both the FROM clause and the permitted-column list.
	var rows []joinRow
	helpers.DoAuthRequestJSON(
		t, base+"/prest-test/public/department"+
			"?_join=inner:employee:department.emp_id:$eq:employee.id"+
			"&_join=inner:employee_badge:department.emp_id:$eq:employee_badge.emp_id"+
			"&_join=inner:employee_secret:department.emp_id:$eq:employee_secret.emp_id"+
			"&_order=d_id",
		nil, http.MethodGet, token, http.StatusOK, "JoinRestrictedRepeated", &rows)

	require.Equal(t, []string{"badge", "d_id", "dept", "emp_id", "id", "name"}, sortedKeys(keysOf(t, rows)))
	require.Equal(t, "Computer", rows[0]["dept"])
	require.Equal(t, "gopher", rows[0]["name"])
	require.Equal(t, "badge-001", rows[0]["badge"])
}

// TestJoinUnrestricted_RepeatedJoin is the same request on the server that runs
// with restrict = false: the second _join must reach the FROM clause there too,
// which is what makes employee_secret's columns show up.
func TestJoinUnrestricted_RepeatedJoin(t *testing.T) {
	base := helpers.ServerURL(t)

	var rows []joinRow
	testutils.DoRequestJSON(
		t, base+"/prest-test/public/department"+
			"?_join=inner:employee:department.emp_id:$eq:employee.id"+
			"&_join=inner:employee_secret:department.emp_id:$eq:employee_secret.emp_id"+
			"&_order=d_id",
		nil, http.MethodGet, http.StatusOK, "JoinUnrestrictedRepeated", &rows)

	keys := keysOf(t, rows)
	for _, col := range []string{"d_id", "dept", "id", "name", "ssn"} {
		require.True(t, keys[col], "column %q missing from unrestricted repeated join: %v", col, rows)
	}
}

// TestJoin_DuplicateTableIsRejected: a _join carries no alias, so joining the
// same table twice builds a statement Postgres refuses. Before this was caught
// up front the request reached the database and answered with the driver's own
// error ("pq: table name \"employee\" specified more than once (42712)");
// it now fails on the join clause itself, on both servers.
func TestJoin_DuplicateTableIsRejected(t *testing.T) {
	duplicateJoin := "/prest-test/public/department" +
		"?_join=inner:employee:department.emp_id:$eq:employee.id" +
		"&_join=left:employee:department.d_id:$eq:employee.id"

	// restrict = true: rejected while the permitted columns are resolved.
	authBase := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, authBase, "test@postgres.rest", "123456")
	helpers.DoAuthRequest(
		t, authBase+duplicateJoin,
		nil, http.MethodGet, token, http.StatusBadRequest, "JoinDuplicate",
		"invalid join clause")

	// restrict = false: rejected while the join clause is built.
	testutils.DoRequest(
		t, helpers.ServerURL(t)+duplicateJoin,
		nil, http.MethodGet, http.StatusBadRequest, "JoinDuplicate",
		"invalid join clause")
}

// TestJoin_DuplicateTableAcrossSchemasIsRejected: the statement exposes a
// joined table under its bare name, so naming a schema does not make a second
// join of the same table distinct.
func TestJoin_DuplicateTableAcrossSchemasIsRejected(t *testing.T) {
	base := helpers.ServerURL(t)

	testutils.DoRequest(
		t, base+"/prest-test/public/department"+
			"?_join=inner:employee:department.emp_id:$eq:employee.id"+
			"&_join=left:public.employee:department.d_id:$eq:employee.id",
		nil, http.MethodGet, http.StatusBadRequest, "JoinDuplicateSchema",
		"invalid join clause")
}

// TestJoinRestricted_SelectCannotWidenPermittedColumns is the negative side of
// the qualified-select syntax: none of the ways a caller might spell a column
// may reach past what the ACL granted. Dropping the qualifier must not make an
// unreadable column look like one of the queried table's, and "<table>.*" must
// not expand a table the config pins to an explicit field list.
func TestJoinRestricted_SelectCannotWidenPermittedColumns(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	const employeeJoin = "?_join=inner:employee:department.emp_id:$eq:employee.id"
	const secretJoin = "?_join=inner:employee_secret:department.emp_id:$eq:employee_secret.emp_id"

	var testCases = []struct {
		description string
		url         string
	}{
		{
			// employee is readable but pinned to fields = ["id", "name"], so
			// asking for all of it must not expand to the whole table.
			"table.* does not expand a table the ACL pins to a field list",
			"/prest-test/public/department" + employeeJoin + "&_select=employee.*",
		},
		{
			// employee_secret has no read permission at all.
			"table.* does not expand a table without read permission",
			"/prest-test/public/department" + secretJoin + "&_select=employee_secret.*",
		},
		{
			// The queried table is pinned to a field list of its own.
			"table.* does not expand the queried table past its field list",
			"/prest-test/public/department" + employeeJoin + "&_select=department.*",
		},
		{
			// A bare * is the same request spelled differently.
			"a bare asterisk does not expand past the field lists",
			"/prest-test/public/department" + secretJoin + "&_select=ssn",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Nothing the caller may read survives, so the request must fail
			// rather than fall back to a wider select list.
			helpers.DoAuthRequest(
				t, base+tc.url,
				nil, http.MethodGet, token, http.StatusBadRequest, "JoinSelectWiden",
				"you don't have permission for this action")
		})
	}

	// The same attempts alongside a column the caller *may* read: the permitted
	// column comes back on its own and the widening attempt is simply dropped,
	// so a caller cannot smuggle one in behind a legitimate one.
	for _, attempt := range []string{"employee.*", "department.*", "ssn"} {
		t.Run("dropped alongside a permitted column: "+attempt, func(t *testing.T) {
			var rows []joinRow
			helpers.DoAuthRequestJSON(
				t, base+"/prest-test/public/department"+secretJoin+
					"&_select=dept,"+attempt+"&_order=d_id",
				nil, http.MethodGet, token, http.StatusOK, "JoinSelectWiden", &rows)

			require.Equal(t, []string{"dept"}, sortedKeys(keysOf(t, rows)))
		})
	}
}

// TestJoin_MalformedClauseIsRejected covers the _join parser's negative paths
// end to end. Every one of these is rejected before a statement is built, so
// none of them can reach the database.
func TestJoin_MalformedClauseIsRejected(t *testing.T) {
	// wantErr pins *why* each clause is rejected, so a case cannot keep passing
	// on an unrelated 400 if the parser changes underneath it.
	var testCases = []struct {
		description string
		join        string
		wantErr     string
	}{
		{"too few arguments", "inner:employee",
			"invalid number of arguments in join statement"},
		{"too many arguments", "inner:employee:department.emp_id:$eq:employee.id:extra",
			"invalid number of arguments in join statement"},
		{"join type outside the whitelist", "weird:employee:department.emp_id:$eq:employee.id",
			"invalid join clause"},
		{"join target is not a valid identifier", "inner:0bad:department.emp_id:$eq:employee.id",
			"invalid identifier"},
		{"comparison operator outside the whitelist", "inner:employee:department.emp_id:$bad:employee.id",
			"invalid operator"},
		{"left operand is not table.column", "inner:employee:emp_id:$eq:employee.id",
			"invalid join clause"},
		{"right operand is not table.column", "inner:employee:department.emp_id:$eq:id",
			"invalid join clause"},
		// Read-only injection attempts through the identifier slots. Asserting
		// on "invalid identifier" is the point: the payload has to be rejected
		// by identifier validation, not by the database choking on it later.
		{"injection through the join target",
			`inner:employee" UNION SELECT current_user--:department.emp_id:$eq:employee.id`,
			"invalid identifier"},
		{"injection through the ON operand",
			`inner:employee:department.emp_id:$eq:employee.id" UNION SELECT current_user--`,
			"invalid identifier"},
	}

	unrestricted := helpers.ServerURL(t)
	authBase := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, authBase, "test@postgres.rest", "123456")

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			query := "?_join=" + url.QueryEscape(tc.join)

			// restrict = false: rejected while the join clause is built.
			testutils.DoRequest(
				t, unrestricted+"/prest-test/public/department"+query,
				nil, http.MethodGet, http.StatusBadRequest, "JoinMalformed", tc.wantErr)

			// restrict = true: the same clause must not become permitted just
			// because column resolution runs first.
			helpers.DoAuthRequest(
				t, authBase+"/prest-test/public/department"+query,
				nil, http.MethodGet, token, http.StatusBadRequest, "JoinMalformed", tc.wantErr)
		})
	}
}

// TestJoin_QualifiedAsteriskSelectRejectsInvalidIdentifier covers the select
// syntax the join support added: "<table>.*" is quoted as an identifier, so a
// prefix that is not one has to be rejected rather than interpolated. These run
// on the unrestricted server, where _select reaches the SQL builder verbatim.
func TestJoin_QualifiedAsteriskSelectRejectsInvalidIdentifier(t *testing.T) {
	base := helpers.ServerURL(t)
	const employeeJoin = "?_join=inner:employee:department.emp_id:$eq:employee.id"

	// Positive control first: the legitimate spelling works, so the rejections
	// below are about the prefix and not about "<table>.*" being unsupported.
	var rows []joinRow
	testutils.DoRequestJSON(
		t, base+"/prest-test/public/department"+employeeJoin+"&_select=employee.*&_order=d_id",
		nil, http.MethodGet, http.StatusOK, "JoinAsteriskSelect", &rows)
	require.Equal(t, []string{"id", "name"}, sortedKeys(keysOf(t, rows)))

	for _, selectField := range []string{
		"0bad.*",                   // prefix is not a valid identifier
		`employee".*`,              // quote escape attempt in the prefix
		"(SELECT current_user).*",  // subquery in the prefix
		"employee.* , pg_sleep(3)", // second field rides in behind the first
	} {
		t.Run(selectField, func(t *testing.T) {
			// Each of these must fail identifier validation, not reach the
			// database and fail there.
			testutils.DoRequest(
				t, base+"/prest-test/public/department"+employeeJoin+
					"&_select="+url.QueryEscape(selectField),
				nil, http.MethodGet, http.StatusBadRequest, "JoinAsteriskSelect",
				"invalid identifier")
		})
	}
}

// TestJoin_QualifiedAsteriskSelectCannotReachAnUnjoinedTable covers the other
// half of "<table>.*": a prefix can be a perfectly valid identifier and still
// name a table the query never joined. "pg_catalog.pg_authid" is exactly that,
// and the two servers refuse it at different points -- worth pinning both,
// because only one of them is pREST's own doing.
func TestJoin_QualifiedAsteriskSelectCannotReachAnUnjoinedTable(t *testing.T) {
	const employeeJoin = "?_join=inner:employee:department.emp_id:$eq:employee.id"
	const catalogSelect = "&_select=" + "pg_catalog.pg_authid.*"

	// restrict = false: _select reaches SQL verbatim, so the prefix is quoted
	// and Postgres rejects the statement for naming a table not in its FROM
	// clause. No row of pg_authid comes back either way.
	testutils.DoRequest(
		t, helpers.ServerURL(t)+"/prest-test/public/department"+employeeJoin+catalogSelect,
		nil, http.MethodGet, http.StatusBadRequest, "JoinAsteriskUnjoined",
		"missing FROM-clause entry")

	// restrict = true: it never gets that far. The prefix matches neither the
	// queried table nor a joined one, so the ACL drops it and nothing is left
	// to select.
	authBase := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, authBase, "test@postgres.rest", "123456")
	helpers.DoAuthRequest(
		t, authBase+"/prest-test/public/department"+employeeJoin+catalogSelect,
		nil, http.MethodGet, token, http.StatusBadRequest, "JoinAsteriskUnjoined",
		"you don't have permission for this action")
}

// TestJoin_RequiresAuth is the baseline for every restricted case above: the
// join query string must not get a request past authentication.
func TestJoin_RequiresAuth(t *testing.T) {
	base := helpers.AuthServerURL(t)

	helpers.DoAuthRequest(
		t, base+departmentEmployeeJoin,
		nil, http.MethodGet, "", http.StatusUnauthorized, "JoinRequiresAuth")
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
