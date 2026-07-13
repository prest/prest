# Integration test documentation examples

## 1. Narrative step comments (preferred for sequential flows)

Source: `integration/controllers/queries_database_test.go`

```go
func TestQueriesDatabaseExecution(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// Test the fulltable/get_all endpoint
	// Expected to succeed and return the body in the expectedBody slice.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/fulltable/get_all?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "QueriesDBExecute")

	// Test the fulltable/get_all endpoint with a database name
	// Expected to succeed and return the body in the expectedBody slice.
	// It will use the database name from the URL.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/prest-test/fulltable/get_all?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "QueriesDBExecuteWithDB")

	// Test the registry endpoint
	// Expected to succeed and return the body in the expectedBody slice.
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry", map[string]string{
		"location": "itest",
		"name":     "ephemeral",
		"read_sql": "SELECT 1",
	}, http.MethodPost, token, http.StatusCreated, "QueriesDBCreateEphemeral")
}
```

Each request has what / expected outcome / optional why. Scenario names match
the comment intent.

## 2. Table-driven `description` (comments optional)

Source: `integration/controllers/crud_test.go`

Per-request block comments may be skipped when every case has a clear
`description`:

```go
func TestGetTables(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Get tables without custom where clause", "/tables", "GET", http.StatusOK},
		{"Get tables with custom where clause", "/tables?c.relname=$eq.test", "GET", http.StatusOK},
		{"Get tables with custom where invalid clause", "/tables?0c.relname=$eq.test", "GET", http.StatusBadRequest},
		{"Get tables with ORDER BY and invalid column", "/tables?_order=0c.relname", "GET", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, "GetTables")
	}
}
```

Descriptions name the scenario and imply success vs failure (invalid clause →
`StatusBadRequest`). Vague labels like `"case 1"` are not acceptable.
