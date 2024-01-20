package middlewares

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"

	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers"
)

var (
	prestCfg = &config.Prest{
		Debug:           true,
		CORSAllowOrigin: []string{"*"},
		AuthTable:       "prest_users",
		AuthUsername:    "username",
		AuthPassword:    "password",
		AuthMetadata:    []string{"first_name", "last_name", "last_login"},
		HTTPPort:        3000,
		Cache: config.CacheConf{
			Enabled: true,
			Endpoints: []config.Endpoint{
				{
					Endpoint: "/prest/public/test",
					Time:     5,
				},
			},
		},
		AccessConf: config.AccessConf{
			Tables: []config.TablesConf{
				{
					Name:        "Reply",
					Permissions: []string{"read", "write", "delete"},
					Fields:      []string{"id", "name"},
				},
				{
					Name:        "test",
					Permissions: []string{"read", "write", "delete"},
					Fields:      []string{"id", "name"},
				},
				{
					Name:        "testarray",
					Permissions: []string{"read", "write", "delete"},
					Fields:      []string{"id", "data"},
				},
				{
					Name:        "test2",
					Permissions: []string{"read", "write", "delete"},
					Fields:      []string{"id", "name"},
				},
				{
					Name:        "test3",
					Permissions: []string{"read", "write", "delete"},
					Fields:      []string{"id", "name"},
				},
				{
					Name:        "test4",
					Permissions: []string{"read", "write", "delete"},
					Fields:      []string{"id", "name"},
				},
				{
					Name:        "test5",
					Permissions: []string{"read", "write", "delete"},
					Fields:      []string{"*"},
				},
				{
					Name:        "test_readonly_access",
					Permissions: []string{"read"},
					Fields:      []string{"id", "name"},
				},
				{
					Name:        "test_write_and_delete_access",
					Permissions: []string{"write", "delete"},
				},
				{
					Name:        "test_list_only_id",
					Permissions: []string{"read"},
					Fields:      []string{"id"},
				},
				{
					Name:        "test6",
					Permissions: []string{"read", "write", "delete"},
					Fields:      []string{"nuveo", "name"},
				},
				{
					Name:        "view_test",
					Permissions: []string{"read"},
					Fields:      []string{"player"},
				},
				{
					Name:        "test_group_by_table",
					Permissions: []string{"read"},
					Fields:      []string{"id", "name", "age", "salary"},
				},
			},
		},
	}
)

func TestGet(t *testing.T) {
	require.NotNil(t, Get(&config.Prest{}, nil))
}

func Test_GetWithReorderedMiddleware(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n := Get(&config.Prest{}, nil, customMiddleware)
	n.UseHandler(r)

	server := httptest.NewServer(n)
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()
	require.Contains(t, string(body), "Calling custom middleware")
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func Test_GetWithoutReorderedMiddleware(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n := Get(&config.Prest{}, nil)
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

func customMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	m := make(map[string]string)
	m["msg"] = "Calling custom middleware"
	b, _ := json.Marshal(m)

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)

	next(w, r)
}

func TestDebug(t *testing.T) {
	nd := appTest(&config.Prest{Debug: true})

	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	resp, err := http.Get(serverd.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestEnableDefaultJWT(t *testing.T) {
	cfg := &config.Prest{
		Debug:            false,
		EnableDefaultJWT: false,
	}

	nd := appTest(cfg)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	resp, err := http.Get(serverd.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
}

func TestJWTIsRequired(t *testing.T) {
	cfg := &config.Prest{
		Debug:            false,
		EnableDefaultJWT: true,
	}

	nd := appTestWithJwt(cfg)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	resp, err := http.Get(serverd.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWTSignatureOk(t *testing.T) {
	bearer := "Bearer eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImpvaG4uZG9lQHNvbWV3aGVyZS5jb20iLCJpYXQiOjE1MTc1NjM2MTYsImlzcyI6InByaXZhdGUiLCJqdGkiOiJjZWZhNzRmZS04OTRjLWZmNjMtZDgxNi00NjIwYjhjZDkyZWUiLCJvcmciOiJwcml2YXRlIiwic3ViIjoiam9obi5kb2UifQ.zLWkEd4hP4XdCD_DlRy6mgPeKwEl1dcdtx5A_jHSfmc87EsrGgNSdi8eBTzCgSU0jgV6ssTgQwzY6x4egze2xA"

	cfg := &config.Prest{
		Debug:            false,
		EnableDefaultJWT: true,
		JWTKey:           "s3cr3t",
		JWTAlgo:          "HS512",
	}

	nd := appTestWithJwt(cfg)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, respd.StatusCode)
}

func TestJWTSignatureKo(t *testing.T) {
	bearer := "Bearer: eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImpvaG4uZG9lQHNvbWV3aGVyZS5jb20iLCJleHAiOjE1MjUzMzk2MTYsImlhdCI6MTUxNzU2MzYxNiwiaXNzIjoicHJpdmF0ZSIsImp0aSI6ImNlZmE3NGZlLTg5NGMtZmY2My1kODE2LTQ2MjBiOGNkOTJlZSIsIm9yZyI6InByaXZhdGUiLCJzdWIiOiJqb2huLmRvZSJ9.zGP1Xths2bK2r9FN0Gv1SzyoisO0dhRwvqrPvunGxUyU5TbkfdnTcQRJNYZzJfGILeQ9r3tbuakWm-NIoDlbbA"

	cfg := &config.Prest{
		Debug:            false,
		EnableDefaultJWT: true,
		JWTKey:           "s3cr3t",
		JWTAlgo:          "HS256",
	}

	nd := appTestWithJwt(cfg)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, respd.StatusCode)
}

func Test_CORS_Middleware(t *testing.T) {

	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("custom route"))
	})

	n := Get(prestCfg, nil)
	n.UseHandler(r)

	server := httptest.NewServer(n)
	defer server.Close()
	req, err := http.NewRequest("OPTIONS", server.URL, nil)
	require.NoError(t, err)

	req.Header.Set("Access-Control-Request-Method", "GET")

	client := http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	require.Equal(t, "OPTIONS", resp.Request.Method)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	var body []byte
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Zero(t, len(body))
}

func TestExposeTablesMiddleware(t *testing.T) {
	cfg := &config.Prest{
		Debug:           true,
		CORSAllowOrigin: []string{"*"},
		ExposeConf: config.ExposeConf{
			Enabled:         true,
			DatabaseListing: false,
			TableListing:    false,
			SchemaListing:   false,
		},
		AuthTable:    "prest_users",
		AuthUsername: "username",
		AuthPassword: "password",
		AuthMetadata: []string{"first_name", "last_name", "last_login"},
	}

	routerCfg, err := controllers.New(prestCfg, nil, nil, nil)
	require.NoError(t, err)

	r := mux.NewRouter()

	r.HandleFunc("/tables", routerCfg.GetTables).Methods("GET")
	r.HandleFunc("/databases", routerCfg.GetDatabases).Methods("GET")
	r.HandleFunc("/schemas", routerCfg.GetSchemas).Methods("GET")

	n := Get(cfg, nil)
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()

	resp, err := http.Get(server.URL + "/tables")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp, err = http.Get(server.URL + "/databases")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp, err = http.Get(server.URL + "/schemas")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func appTest(cfg *config.Prest) *negroni.Negroni {
	n := Get(cfg, nil)
	r := mux.NewRouter()
	if !cfg.Debug && !cfg.EnableDefaultJWT {
		n.UseHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})
	}

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")

	n.UseHandler(r)
	return n
}

func appTestWithJwt(cfg *config.Prest) *negroni.Negroni {
	n := Get(cfg, nil)
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")

	n.UseHandler(r)
	return n
}
