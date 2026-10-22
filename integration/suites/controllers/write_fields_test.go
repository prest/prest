package controllers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

// writeFieldsRaw sends an authenticated request and returns status and raw
// body, for assertions on the exact bytes of a write response.
func writeFieldsRaw(t *testing.T, target string, body interface{}, method, token string) (int, string) {
	t.Helper()
	var buf []byte
	if body != nil {
		var err error
		buf, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req, err := http.NewRequest(method, target, bytes.NewReader(buf))
	require.NoError(t, err)
	req.Header.Set("X-Application", "prest")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(out)
}

func writeFieldsNameFilter(name string) string {
	return "?name=" + url.QueryEscape("$eq."+name)
}

// TestWriteFieldAllowlist_RejectsUnwritableColumns: with access.restrict=true,
// a body key outside [[access.tables]] fields must be rejected with 400 and
// nothing may be written, not even the allowed columns of the same body.
func TestWriteFieldAllowlist_RejectsUnwritableColumns(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")
	table := base + "/prest-test/public/write_fields_restricted"
	batch := base + "/batch/prest-test/public/write_fields_restricted"
	n := fmt.Sprintf("wf-%d", time.Now().UnixNano())
	auth := map[string]string{"Authorization": "Bearer " + token}

	t.Cleanup(func() {
		for _, s := range []string{n, n + "-x", n + "-r", n + "-b1", n + "-b2"} {
			writeFieldsRaw(t, table+writeFieldsNameFilter(s), nil, http.MethodDelete, token)
		}
	})

	// POST an allowed column only. Expected: 201. Control for the rejections below.
	helpers.DoAuthRequest(t, table, map[string]interface{}{"name": n},
		http.MethodPost, token, http.StatusCreated, "WFControlInsert", n)

	// POST a body setting the non-writable "role". Expected: 400 "not writable".
	// Why: otherwise a caller could self-assign privileged columns (mass assignment).
	helpers.DoAuthRequest(t, table, map[string]interface{}{"name": n + "-x", "role": "admin"},
		http.MethodPost, token, http.StatusBadRequest, "WFInsertRole", "not writable")

	// GET the row the rejected POST would have created. Expected: 200 and [].
	// Why: a rejected request must not partially write the allowed column.
	var rows []map[string]interface{}
	helpers.DoAuthRequestJSON(t, table+writeFieldsNameFilter(n+"-x"), nil,
		http.MethodGet, token, http.StatusOK, "WFNoPartialInsert", &rows)
	require.Empty(t, rows)

	// PUT and PATCH setting "role" on the control row (advisory repro).
	// Expected: 400 for both.
	for _, m := range []string{http.MethodPut, http.MethodPatch} {
		helpers.DoAuthRequest(t, table+writeFieldsNameFilter(n),
			map[string]interface{}{"role": "superadmin"},
			m, token, http.StatusBadRequest, "WFUpdateRole"+m, "not writable")
	}

	// PATCH a body mixing an allowed column (name) with "role". Expected: 400.
	helpers.DoAuthRequest(t, table+writeFieldsNameFilter(n),
		map[string]interface{}{"name": n + "-r", "role": "admin"},
		http.MethodPatch, token, http.StatusBadRequest, "WFPatchMixed", "not writable")

	// GET the control row by its original name. Expected: still exactly 1 row.
	// Why: the allowed "name" column must not have been renamed by the rejected PATCH.
	rows = nil
	helpers.DoAuthRequestJSON(t, table+writeFieldsNameFilter(n), nil,
		http.MethodGet, token, http.StatusOK, "WFNotRenamed", &rows)
	require.Len(t, rows, 1)

	// Batch POST where the second item sets "role". Expected: 400.
	// Why: one bad item must reject the whole batch.
	items := []map[string]interface{}{
		{"name": n + "-b1"},
		{"name": n + "-b2", "role": "admin"},
	}
	helpers.DoAuthRequest(t, batch, items,
		http.MethodPost, token, http.StatusBadRequest, "WFBatchRole", "not writable")

	// Same batch through the COPY path (Prest-Batch-Method: copy). Expected: 400.
	// Why: COPY builds its column list separately from the INSERT path.
	testutils.DoRequestWithHeaders(t, batch, items, http.MethodPost, http.StatusBadRequest,
		"WFBatchCopyRole",
		map[string]string{"Authorization": auth["Authorization"], "Prest-Batch-Method": "copy"},
		"not writable")

	// GET the first batch item. Expected: 200 and [] (no partial batch insert).
	rows = nil
	helpers.DoAuthRequestJSON(t, table+writeFieldsNameFilter(n+"-b1"), nil,
		http.MethodGet, token, http.StatusOK, "WFNoPartialBatch", &rows)
	require.Empty(t, rows)

	// POST with ?_select=role and body {"role":"admin"}. Expected: 400.
	// Why: query params must not widen the write allowlist.
	helpers.DoAuthRequest(t, table+"?_select=role", map[string]interface{}{"role": "admin"},
		http.MethodPost, token, http.StatusBadRequest, "WFSelectWiden", "not writable")
}

// TestWriteFieldAllowlist_ResponsesEchoOnlyReadableColumns: write responses
// must drop columns the caller may not read, keeping the usual JSON shape.
func TestWriteFieldAllowlist_ResponsesEchoOnlyReadableColumns(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")
	table := base + "/prest-test/public/write_fields_restricted"
	batch := base + "/batch/prest-test/public/write_fields_restricted"
	only := base + "/prest-test/public/write_only_restricted"
	n := fmt.Sprintf("wf-%d", time.Now().UnixNano())
	w := n + "-wo"

	t.Cleanup(func() {
		for _, s := range []string{n, n + "-b1", n + "-b2"} {
			writeFieldsRaw(t, table+writeFieldsNameFilter(s), nil, http.MethodDelete, token)
		}
		writeFieldsRaw(t, only+writeFieldsNameFilter(w), nil, http.MethodDelete, token)
	})

	// POST an allowed column. Expected: 201 object with id and name, no role.
	// Why: role has a default and is not readable, so it must not be echoed.
	var obj map[string]interface{}
	helpers.DoAuthRequestJSON(t, table, map[string]interface{}{"name": n},
		http.MethodPost, token, http.StatusCreated, "WFEchoInsert", &obj)
	require.Contains(t, obj, "id")
	require.Contains(t, obj, "name")
	require.NotContains(t, obj, "role")

	// PATCH with _returning=*. Expected: 200 array, rows have name but no role.
	// Why: "*" must be narrowed to the readable columns.
	var rows []map[string]interface{}
	helpers.DoAuthRequestJSON(t, table+writeFieldsNameFilter(n)+"&_returning=*",
		map[string]interface{}{"name": n},
		http.MethodPatch, token, http.StatusOK, "WFReturningStar", &rows)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Contains(t, r, "name")
		require.NotContains(t, r, "role")
	}

	// PATCH with _returning=role. Expected: 200 array of empty objects.
	// Why: explicitly asking for an unreadable column must not leak it.
	rows = nil
	helpers.DoAuthRequestJSON(t, table+writeFieldsNameFilter(n)+"&_returning=role",
		map[string]interface{}{"name": n},
		http.MethodPatch, token, http.StatusOK, "WFReturningRole", &rows)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Empty(t, r)
	}

	// Batch POST of two allowed rows. Expected: 201 array of 2, no role in any row.
	rows = nil
	helpers.DoAuthRequestJSON(t, batch,
		[]map[string]interface{}{{"name": n + "-b1"}, {"name": n + "-b2"}},
		http.MethodPost, token, http.StatusCreated, "WFEchoBatch", &rows)
	require.Len(t, rows, 2)
	for _, r := range rows {
		require.NotContains(t, r, "role")
	}

	// DELETE with _returning=*. Expected: 200 array of deleted rows without role.
	rows = nil
	helpers.DoAuthRequestJSON(t, table+writeFieldsNameFilter(n)+"&_returning=*", nil,
		http.MethodDelete, token, http.StatusOK, "WFDeleteReturning", &rows)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.NotContains(t, r, "role")
	}

	// POST to the write-only table (no read permission, no fields list).
	// Expected: 201 and the body is exactly {}.
	// Why: nothing is readable there, so no column may be echoed.
	status, body := writeFieldsRaw(t, only,
		map[string]interface{}{"name": w, "role": "admin"}, http.MethodPost, token)
	require.Equal(t, http.StatusCreated, status)
	require.Equal(t, "{}", strings.TrimSpace(body))

	// DELETE the write-only row. Expected: 200 with rows_affected.
	helpers.DoAuthRequest(t, only+writeFieldsNameFilter(w), nil,
		http.MethodDelete, token, http.StatusOK, "WFWriteOnlyCleanup", "rows_affected")
}
