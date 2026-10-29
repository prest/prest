package controllers

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/prest/prest/v2/adapters"
)

// columnNotReadableMsg is the 403 a read request gets when it filters, groups,
// orders or counts by a column the caller cannot read (GHSA-48r9-g6f9-x3gm).
const columnNotReadableMsg = "column %q is not readable on this table"

// unreadableColumn returns the first column the request references outside
// _select that the field ACL does not let userName read, or "". Readability is
// decided by the adapter's own FieldsPermissions, asked about one column at a
// time, so the ACL rules (bare names belong to the queried table, t.col to the
// joined table t, reserved tables, restrict=false, user overrides) stay in one
// place. A builder that does not list request columns gets no check.
func (h *CRUDHandler) unreadableColumn(r *http.Request, database, schema, table, userName string) (string, error) {
	lister, ok := h.builder.(adapters.RequestColumnsLister)
	if !ok {
		return "", nil
	}
	refs, err := lister.RequestColumns(r)
	if err != nil {
		return "", err
	}
	readable := func(col string) (bool, error) {
		fields, err := h.perms.FieldsPermissions(columnRequest(r, col), database, schema, table, "read", userName)
		return len(fields) > 0, err
	}
	// A queried table granting "*" lets a bare name through, but SQL resolves
	// an unqualified name to whichever joined table has the column, so it must
	// then be readable on every joined table as well.
	wildcard := false
	if len(refs.JoinTables) > 0 {
		all, err := h.perms.FieldsPermissions(columnRequest(r, ""), database, schema, table, "read", userName)
		if err != nil {
			return "", err
		}
		wildcard = slices.Contains(all, "*")
	}
	for _, col := range refs.Columns {
		if ok, err := readable(col); err != nil || !ok {
			return col, err
		}
		if !wildcard || strings.Contains(col, ".") {
			continue
		}
		for _, joined := range refs.JoinTables {
			if ok, err := readable(joined + "." + col); err != nil || !ok {
				return col, err
			}
		}
	}
	return "", nil
}

// columnRequest clones r keeping only its _join clauses and, when col is set,
// _select=col, so FieldsPermissions answers for exactly that column.
func columnRequest(r *http.Request, col string) *http.Request {
	q := url.Values{}
	if joins := r.URL.Query()["_join"]; len(joins) > 0 {
		q["_join"] = joins
	}
	if col != "" {
		q.Set("_select", col)
	}
	return withQuery(r, q)
}

// countRequest clones r with _select replaced by the ACL-filtered cols, so
// CountByRequest, which re-reads _select, can never project a column the
// normal select list drops.
func countRequest(r *http.Request, cols []string) *http.Request {
	q := r.URL.Query()
	q.Set("_select", strings.Join(cols, ","))
	return withQuery(r, q)
}

func withQuery(r *http.Request, q url.Values) *http.Request {
	req := r.Clone(r.Context())
	u := *req.URL
	u.RawQuery = q.Encode()
	req.URL = &u
	return req
}
