package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/prest/prest/config"
	"github.com/stretchr/testify/require"
)

func Test_ValidGetScript(t *testing.T) {
	var testCases = []struct {
		description string
		method      string
		path        string
		file        string
		err         error
	}{
		{"Get script file by GET Method", "GET", "fulltable", "get_all", nil},
		{"Get script file by POST Method", "POST", "fulltable", "write_all", nil},
		{"Get script file by PUT Method", "PUT", "fulltable", "put_all", nil},
		{"Get script file by PATCH Method", "PATCH", "fulltable", "patch_all", nil},
		{"Get script file by DELETE Method", "DELETE", "fulltable", "delete_all", nil},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{
			QueriesPath: "../../testdata/queries",
		})
		_, err := adpt.GetScript(tc.method, tc.path, tc.file)
		if err != tc.err {
			t.Errorf("expected no errors, but got %s", err)
		}
	}
}

func Test_InvalidGetScript(t *testing.T) {
	var testCases = []struct {
		description string
		method      string
		path        string
		file        string
	}{
		{"Try get a script with INVALID HTTP Method", "ANY", "fulltable", "delete_all"},
		{"Try get a script noexistent", "GET", "fulltable", "dloohot_all"},
		{"Try get a script with nonexistent folder", "GET", "sasalla", "get_all"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{
			QueriesPath: "../../testdata/queries",
		})
		_, err := adpt.GetScript(tc.method, tc.path, tc.file)
		if err == nil {
			t.Errorf("expected no error, but got %s", err)
		}
	}
}

func Test_ParseScriptInvalid(t *testing.T) {
	templateData := map[string]interface{}{}
	templateData["field1"] = "abc"

	t.Log("Parse script with get_all file")

	adpt := NewAdapter(&config.Prest{
		QueriesPath: "../../testdata/queries",
	})
	sql, _, err := adpt.ParseScript(
		fmt.Sprintf(adpt.cfg.QueriesPath+"/fulltable/%s", "get_all.read.sql"),
		templateData)
	if err != nil {
		t.Errorf("expected no error, but got: %v", err)
	}

	if sql != "SELECT * FROM test7 WHERE name = 'abc'" {
		t.Errorf("SQL unexpected, got: %s", sql)
	}
}

func Test_ParseScriptSyntaxInvalid(t *testing.T) {
	templateData := map[string]interface{}{}
	templateData["field1"] = 1

	adpt := NewAdapter(&config.Prest{
		QueriesPath: "../../testdata/queries",
	})
	scriptPath := fmt.Sprintf(
		adpt.cfg.QueriesPath+"/fulltable/%s", "parse_syntax_invalid.read.sql")

	_, _, err := adpt.ParseScript(scriptPath, templateData)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not parse file")
}

func Test_ParseScript(t *testing.T) {
	templateData := map[string]interface{}{}
	templateData["field1"] = []string{"abc", "test"}

	t.Log("Parse script with get_all_slice file")

	adpt := NewAdapter(&config.Prest{
		QueriesPath: "../../testdata/queries",
	})
	scriptPath := fmt.Sprintf(
		adpt.cfg.QueriesPath+"/fulltable/%s", "get_all_slice.read.sql")

	sql, _, err := adpt.ParseScript(scriptPath, templateData)
	require.NoError(t, err)
	require.Equal(t, "SELECT * FROM test7 WHERE name IN ('abc', 'test')", sql)
}

func TestWriteSQL(t *testing.T) {
	var testValidCases = []struct {
		description string
		sql         string
		values      []interface{}
		pass        bool
	}{
		{"Execute a valid INSERT sql", "INSERT INTO test7(name) values ('lulu')", []interface{}{}, true},
		{"Execute a valid UPDATE sql", "UPDATE test7 SET name = 'lulu' WHERE surname = 'temer'", []interface{}{}, true},
		{"Execute a valid DELETE sql", "DELETE FROM test7 WHERE name = 'lulu'", []interface{}{}, true},
		{"Execute a valid DELETE sql", "DELETE FROM test7 WHERE name = 'lulu'", []interface{}{1, 2}, false},
	}
	for _, tc := range testValidCases {
		t.Log(tc.description)
		// todo: fix this test
		adpt := NewAdapter(&config.Prest{})

		sc := adpt.WriteSQL(tc.sql, tc.values)
		if sc.Err() != nil && tc.pass {
			t.Errorf("pass true, got: %s", sc.Err())
		} else if sc.Err() == nil && !tc.pass {
			t.Errorf("pass false, got: %s", sc.Err())
		}
	}
}

func TestWriteSQLCtx(t *testing.T) {
	ctx := context.Background()

	var testValidCases = []struct {
		description string
		sql         string
		values      []interface{}
		pass        bool
	}{
		{"Execute a valid INSERT sql", "INSERT INTO test7(name) values ('lulu')", []interface{}{}, true},
		{"Execute a valid UPDATE sql", "UPDATE test7 SET name = 'lulu' WHERE surname = 'temer'", []interface{}{}, true},
		{"Execute a valid DELETE sql", "DELETE FROM test7 WHERE name = 'lulu'", []interface{}{}, true},
		{"Execute a valid DELETE sql", "DELETE FROM test7 WHERE name = 'lulu'", []interface{}{1, 2}, false},
	}
	for _, tc := range testValidCases {
		t.Log(tc.description)
		// todo: fix this test
		adpt := NewAdapter(&config.Prest{})

		sc := adpt.WriteSQLCtx(ctx, tc.sql, tc.values)
		if sc.Err() != nil && tc.pass {
			t.Errorf("pass true, got: %s", sc.Err())
		} else if sc.Err() == nil && !tc.pass {
			t.Errorf("pass false, got: %s", sc.Err())
		}
	}
}

func TestExecuteScripts(t *testing.T) {
	var testCases = []struct {
		description string
		method      string
		sql         string
		values      []interface{}
		err         error
	}{
		{"Get result with GET HTTP Method", "GET", "SELECT * FROM test7", []interface{}{}, nil},
		{"Get result with POST HTTP Method", "POST", "INSERT INTO test7 (name) VALUES ('lala')", []interface{}{}, nil},
		{"Get result with PUT HTTP Method", "PUT", "UPDATE test7 SET name = 'lala' WHERE surname = 'temer'", []interface{}{}, nil},
		{"Get result with PATCH HTTP Method", "PATCH", "UPDATE test7 SET surname = 'temer' WHERE name = 'lala'", []interface{}{}, nil},
		{"Get result with DELETE HTTP Method", "DELETE", "DELETE FROM test7 WHERE surname = 'lala'", []interface{}{}, nil},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		sc := adpt.ExecuteScripts(tc.method, tc.sql, tc.values)
		if tc.err != sc.Err() {
			t.Errorf("expected no errors, but got %s", sc.Err())
		}
	}

	t.Log("Get errors with invalid HTTP Method")
	values := make([]interface{}, 0)
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.ExecuteScripts("ANY", "SELECT * FROM test7", values)
	if len(sc.Bytes()) > 0 {
		t.Errorf("expected empty result, but got %s", sc.Bytes())
	}

	if sc.Err() == nil {
		t.Errorf("expected errors, but got %s", sc.Err())
	}
}

func TestExecuteScriptsCtx(t *testing.T) {
	ctx := context.Background()

	var testCases = []struct {
		description string
		method      string
		sql         string
		values      []interface{}
		err         error
	}{
		{"Get result with GET HTTP Method", "GET", "SELECT * FROM test7", []interface{}{}, nil},
		{"Get result with POST HTTP Method", "POST", "INSERT INTO test7 (name) VALUES ('lala')", []interface{}{}, nil},
		{"Get result with PUT HTTP Method", "PUT", "UPDATE test7 SET name = 'lala' WHERE surname = 'temer'", []interface{}{}, nil},
		{"Get result with PATCH HTTP Method", "PATCH", "UPDATE test7 SET surname = 'temer' WHERE name = 'lala'", []interface{}{}, nil},
		{"Get result with DELETE HTTP Method", "DELETE", "DELETE FROM test7 WHERE surname = 'lala'", []interface{}{}, nil},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		sc := adpt.ExecuteScriptsCtx(ctx, tc.method, tc.sql, tc.values)
		require.Equal(t, tc.err, sc.Err())
	}

	t.Log("Get errors with invalid HTTP Method")
	values := make([]interface{}, 0)
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.ExecuteScriptsCtx(ctx, "ANY", "SELECT * FROM test7", values)
	require.Len(t, sc.Bytes(), 0)
	require.Error(t, sc.Err())
}

func TestParseFuncLimitOffset(t *testing.T) {
	templateData := map[string]interface{}{}

	scriptPath := fmt.Sprint(os.Getenv("PREST_QUERIES_LOCATION"), "/fulltable/%s")

	t.Log("Parse script with limitoffset file")

	adpt := NewAdapter(&config.Prest{})

	sql, _, err := adpt.ParseScript(fmt.Sprintf(scriptPath, "limitoffset.read.sql"), templateData)
	require.NoError(t, err)
	require.Equal(t, "SELECT * FROM test7 LIMIT 10 OFFSET(1 - 1) * 10", sql)
}
