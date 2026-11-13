package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/stretchr/testify/require"
)

// listerBuilder is a request builder that also lists the columns a request
// references, as the Postgres, TimescaleDB and MySQL adapters do.
type listerBuilder struct {
	*mockgen.MockRequestQueryBuilder
	refs adapters.RequestColumns
}

func (b listerBuilder) RequestColumns(*http.Request) (adapters.RequestColumns, error) {
	return b.refs, nil
}

var _ adapters.RequestColumnsLister = listerBuilder{}

// aclPerms answers FieldsPermissions like an adapter would for one table:
// with _select=c it returns c when readable[c], nothing otherwise; without
// _select it returns all, the table's permitted list.
func aclPerms(ctrl *gomock.Controller, all []string, readable map[string]bool) *mockgen.MockPermissionsChecker {
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "db", "public", "department", "read", "").
		DoAndReturn(func(r *http.Request, _, _, _, _, _ string) ([]string, error) {
			col := r.URL.Query().Get("_select")
			if col == "" {
				return all, nil
			}
			if readable[col] {
				return []string{col}, nil
			}
			return nil, nil
		}).AnyTimes()
	return perms
}

func refsHandler(ctrl *gomock.Controller, perms adapters.PermissionsChecker, refs adapters.RequestColumns) *CRUDHandler {
	builder := listerBuilder{MockRequestQueryBuilder: mockgen.NewMockRequestQueryBuilder(ctrl), refs: refs}
	return NewCRUDHandler(Deps{Perms: perms, Builder: builder})
}

func departmentRequest(query string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/db/public/department"+query, nil)
}

// A builder without the lister keeps today's behaviour: no reference check.
func TestUnreadableColumn_NoLister(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	h := NewCRUDHandler(Deps{Perms: mockgen.NewMockPermissionsChecker(ctrl), Builder: mockgen.NewMockRequestQueryBuilder(ctrl)})
	col, err := h.unreadableColumn(departmentRequest("?ssn=$eq.1"), "db", "public", "department", "")
	require.NoError(t, err)
	require.Empty(t, col)
}

// Every referenced column readable: the request goes through.
func TestUnreadableColumn_AllReadable(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	perms := aclPerms(ctrl, []string{"dept"}, map[string]bool{"dept": true, "employee.name": true})
	h := refsHandler(ctrl, perms, adapters.RequestColumns{Columns: []string{"dept", "employee.name"}, JoinTables: []string{"employee"}})
	col, err := h.unreadableColumn(departmentRequest(""), "db", "public", "department", "")
	require.NoError(t, err)
	require.Empty(t, col)
}

// The first unreadable reference is reported, the advisory's case.
func TestUnreadableColumn_FirstUnreadable(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	perms := aclPerms(ctrl, []string{"dept"}, map[string]bool{"dept": true})
	h := refsHandler(ctrl, perms, adapters.RequestColumns{Columns: []string{"dept", "employee_secret.ssn"}, JoinTables: []string{"employee_secret"}})
	col, err := h.unreadableColumn(departmentRequest(""), "db", "public", "department", "")
	require.NoError(t, err)
	require.Equal(t, "employee_secret.ssn", col)
}

// When the queried table grants "*", a bare name may resolve in SQL to a
// restricted joined table, so it must be readable there too.
func TestUnreadableColumn_BareNameWildcardJoin(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	perms := aclPerms(ctrl, []string{"*"}, map[string]bool{"ssn": true})
	h := refsHandler(ctrl, perms, adapters.RequestColumns{Columns: []string{"ssn"}, JoinTables: []string{"employee_secret"}})
	col, err := h.unreadableColumn(departmentRequest(""), "db", "public", "department", "")
	require.NoError(t, err)
	require.Equal(t, "ssn", col)
}

// An explicit grant on the queried table resolves a bare name there: no
// joined-table lookup is needed.
func TestUnreadableColumn_BareNameExplicitAllow(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	perms := aclPerms(ctrl, []string{"dept"}, map[string]bool{"dept": true})
	h := refsHandler(ctrl, perms, adapters.RequestColumns{Columns: []string{"dept"}, JoinTables: []string{"employee_secret"}})
	col, err := h.unreadableColumn(departmentRequest(""), "db", "public", "department", "")
	require.NoError(t, err)
	require.Empty(t, col)
}

// A permission lookup error is returned, not swallowed.
func TestUnreadableColumn_PermissionError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "db", "public", "department", "read", "").
		Return(nil, errors.New("acl down"))
	h := refsHandler(ctrl, perms, adapters.RequestColumns{Columns: []string{"dept"}})
	_, err := h.unreadableColumn(departmentRequest(""), "db", "public", "department", "")
	require.EqualError(t, err, "acl down")
}

// columnRequest asks about one column: only _join survives next to _select.
func TestColumnRequest_KeepsOnlyJoinAndSelect(t *testing.T) {
	t.Parallel()

	r := departmentRequest("?dept=$eq.x&_order=dept&_join=inner:a:b.c:$eq:a.c&_join=left:d:b.e:$eq:d.e&_select=x")
	q := columnRequest(r, "employee.name").URL.Query()
	require.Equal(t, "employee.name", q.Get("_select"))
	require.Equal(t, []string{"inner:a:b.c:$eq:a.c", "left:d:b.e:$eq:d.e"}, q["_join"])
	require.Len(t, q, 2)
	require.Equal(t, "dept=$eq.x&_order=dept&_join=inner:a:b.c:$eq:a.c&_join=left:d:b.e:$eq:d.e&_select=x", r.URL.RawQuery, "original untouched")

	require.Empty(t, columnRequest(r, "").URL.Query().Get("_select"))
}

// countRequest swaps _select for the ACL-filtered list and keeps the rest.
func TestCountRequest_ReplacesSelect(t *testing.T) {
	t.Parallel()

	r := departmentRequest("?_select=dept,employee_secret.ssn&_count=*&_groupby=dept")
	q := countRequest(r, []string{"department.dept"}).URL.Query()
	require.Equal(t, "department.dept", q.Get("_select"))
	require.Equal(t, "*", q.Get("_count"))
	require.Equal(t, "dept", q.Get("_groupby"))
	require.Equal(t, "dept,employee_secret.ssn", r.URL.Query().Get("_select"), "original untouched")
}

// Select rejects a reference to an unreadable column with 403 before any
// query runs (strict mocks: no executor call is expected).
func TestCRUDHandler_Select_UnreadableReferenceIs403(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "department", "read", "").
		DoAndReturn(func(r *http.Request, _, _, _, _, _ string) ([]string, error) {
			if r.URL.Query().Get("_select") == "employee_secret.ssn" {
				return nil, nil
			}
			return []string{"department.dept"}, nil
		}).AnyTimes()
	builder := listerBuilder{
		MockRequestQueryBuilder: mockgen.NewMockRequestQueryBuilder(ctrl),
		refs:                    adapters.RequestColumns{Columns: []string{"dept", "employee_secret.ssn"}, JoinTables: []string{"employee_secret"}},
	}
	h := NewCRUDHandler(Deps{Perms: perms, Builder: builder, SQL: mockgen.NewMockSQLBuilder(ctrl), Executor: mockgen.NewMockQueryExecutor(ctrl), DB: mockDatabaseRegistry(ctrl)})

	req := crudRequest(http.MethodGet, "/prest-test/public/department?_select=dept,employee_secret.ssn&_count=*&_groupby=dept,employee_secret.ssn", map[string]string{
		"database": "prest-test", "schema": "public", "table": "department",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `column \"employee_secret.ssn\" is not readable on this table`)
}

// selectParam matches a request whose _select query parameter is want.
type selectParam string

func (m selectParam) Matches(x any) bool {
	r, ok := x.(*http.Request)
	return ok && r.URL.Query().Get("_select") == string(m)
}

func (m selectParam) String() string { return "request with _select=" + string(m) }

// selectCountMocks wires a Select whose count query is built from the request
// CountByRequest receives; want is the _select that request must carry.
func selectCountMocks(t *testing.T, ctrl *gomock.Controller, url, wantSelect string) *httptest.ResponseRecorder {
	t.Helper()
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "department", "read", "").Return([]string{"department.dept"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"department.dept"}).Return(`"department"."dept"`, nil)
	sqlBuilder.EXPECT().SelectSQL(gomock.Any(), "prest-test", "public", "department").Return(`SELECT 1`).Times(2)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(selectParam(wantSelect)).Return(`SELECT COUNT(*) FROM`, nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"count":2}]`))
	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodGet, url, map[string]string{"database": "prest-test", "schema": "public", "table": "department"})
	rec := httptest.NewRecorder()
	h.Select(rec, req)
	return rec
}

// _count builds its projection from the ACL-filtered columns, never from the
// raw _select (GHSA-48r9-g6f9-x3gm).
func TestCRUDHandler_Select_CountUsesPermittedColumns(t *testing.T) {
	t.Parallel()

	rec := selectCountMocks(t, gomock.NewController(t),
		"/prest-test/public/department?_select=dept,employee_secret.ssn&_count=*", "department.dept")
	require.Equal(t, http.StatusOK, rec.Code)
}

// Without _select, CountByRequest sees the request unchanged (COUNT only).
func TestCRUDHandler_Select_CountWithoutSelectUnchanged(t *testing.T) {
	t.Parallel()

	rec := selectCountMocks(t, gomock.NewController(t), "/prest-test/public/department?_count=*", "")
	require.Equal(t, http.StatusOK, rec.Code)
}
