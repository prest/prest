package middlewares

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/prest/prest/config/router"
	"github.com/prest/prest/controllers"
	"github.com/urfave/negroni"
)

func init() {
	config.Load()
	postgres.Load()
}

func TestInitApp(t *testing.T) {
	app = nil
	initApp()
	if app == nil {
		t.Errorf("app should not be nil")
	}
	MiddlewareStack = []negroni.Handler{}
}

func TestGetApp(t *testing.T) {
	app = nil
	n := GetApp()
	if n == nil {
		t.Errorf("should return an app")
	}
	MiddlewareStack = []negroni.Handler{}
}

func TestGetAppWithReorderedMiddleware(t *testing.T) {
	app = nil
	MiddlewareStack = []negroni.Handler{
		negroni.Handler(negroni.HandlerFunc(customMiddleware)),
	}
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal("expected run without errors but was", err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("expected run without errors but was", err.Error())
	}
	defer resp.Body.Close()
	if !strings.Contains(string(body), "Calling custom middleware") {
		t.Error("do not contains 'Calling custom middleware'")
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type should application/json but wasn't")
	}
	MiddlewareStack = []negroni.Handler{}
}

func TestGetAppWithoutReorderedMiddleware(t *testing.T) {
	app = nil
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)

	if err != nil {
		t.Fatal("Expected run without errors but was", err.Error())
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type should be application/json but not was", resp.Header.Get("Content-Type"))
	}
	MiddlewareStack = []negroni.Handler{}
}

func TestMiddlewareAccessNoblockingCustomRoutes(t *testing.T) {
	os.Setenv("PREST_DEBUG", "true")
	config.Load()
	postgres.Load()
	app = nil
	r := router.Get()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)

	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")

	r.PathPrefix("/").Handler(negroni.New(
		AccessControl(),
		negroni.Wrap(crudRoutes),
	))
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal("expected run without errors but was", err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("expected run without errors but was", err.Error())
	}
	defer resp.Body.Close()
	if !strings.Contains(string(body), "custom route") {
		t.Error("do not contains 'custom route'")
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type should be application/json but was", resp.Header.Get("Content-Type"))
	}
	resp, err = http.Get(server.URL + "/prest/public/test_write_and_delete_access")
	if err != nil {
		t.Fatal("expected run without errors but was", err.Error())
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("expected run without errors but was", err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("content type should be http.StatusUnauthorized but was %s", resp.Status)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type should be application/json but wasn't")
	}
	if !strings.Contains(string(body), "required authorization to table") {
		t.Error("do not contains 'required authorization to table'")
	}
	MiddlewareStack = []negroni.Handler{}
	os.Setenv("PREST_CONF", "")
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
	os.Setenv("PREST_DEBUG", "true")
	config.Load()
	nd := appTest()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()
	respd, err := http.Get(serverd.URL)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, but got %d", respd.StatusCode)
	}
}

func TestEnableDefaultJWT(t *testing.T) {
	app = nil
	os.Setenv("PREST_JWT_DEFAULT", "false")
	os.Setenv("PREST_DEBUG", "false")
	config.Load()
	nd := appTest()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()
	respd, err := http.Get(serverd.URL)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusNotImplemented {
		t.Errorf("expected status code 501, but got %d", respd.StatusCode)
	}
}

func TestJWTIsRequired(t *testing.T) {
	MiddlewareStack = []negroni.Handler{}
	app = nil
	os.Setenv("PREST_JWT_DEFAULT", "true")
	os.Setenv("PREST_DEBUG", "false")
	config.Load()
	nd := appTestWithJwt()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	respd, err := http.Get(serverd.URL)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status code 401, but got %d", respd.StatusCode)
	}
}

func TestJWTSignatureOk(t *testing.T) {
	app = nil
	MiddlewareStack = nil
	bearer := "Bearer eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImpvaG4uZG9lQHNvbWV3aGVyZS5jb20iLCJpYXQiOjE1MTc1NjM2MTYsImlzcyI6InByaXZhdGUiLCJqdGkiOiJjZWZhNzRmZS04OTRjLWZmNjMtZDgxNi00NjIwYjhjZDkyZWUiLCJvcmciOiJwcml2YXRlIiwic3ViIjoiam9obi5kb2UifQ.zLWkEd4hP4XdCD_DlRy6mgPeKwEl1dcdtx5A_jHSfmc87EsrGgNSdi8eBTzCgSU0jgV6ssTgQwzY6x4egze2xA"
	os.Setenv("PREST_JWT_DEFAULT", "true")
	os.Setenv("PREST_DEBUG", "false")
	os.Setenv("PREST_JWT_KEY", "s3cr3t")
	os.Setenv("PREST_JWT_ALGO", "HS512")
	config.Load()
	nd := appTestWithJwt()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, but got %d", respd.StatusCode)
	}
}

func TestJWTSignatureRSAOk(t *testing.T) {
	const cert string =
		"-----BEGIN CERTIFICATE-----\n" +
		"MIIFtTCCA52gAwIBAgIUDo1bin5Ca1i7eAHaGRMQI4iKWXEwDQYJKoZIhvcNAQEL\n" +
		"BQAwajELMAkGA1UEBhMCR0IxDzANBgNVBAgMBkxvbmRvbjEPMA0GA1UEBwwGTG9u\n" +
		"ZG9uMQwwCgYDVQQKDANaZW4xFjAUBgNVBAsMDUlUIERlcGFydG1lbnQxEzARBgNV\n" +
		"BAMMCnR1cmJjYS5jb20wHhcNMjEwNjAyMTQzMDEwWhcNMjEwNjA1MTQzMDEwWjBq\n" +
		"MQswCQYDVQQGEwJHQjEPMA0GA1UECAwGTG9uZG9uMQ8wDQYDVQQHDAZMb25kb24x\n" +
		"DDAKBgNVBAoMA1plbjEWMBQGA1UECwwNSVQgRGVwYXJ0bWVudDETMBEGA1UEAwwK\n" +
		"dHVyYmNhLmNvbTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBAMsQ7dYF\n" +
		"BlcWU2TcnI2E4SXcJkgXqtZw7SOrl43xXgIxRyh3VN2vdaG8eoyl3Q69Z60Nooia\n" +
		"i45HidiZZM57uXseRCY4cxTTR/aEmfUv7W/ueWrOj89ZBFhROcemSFV81AS17ynt\n" +
		"BoUtqUX/9Nzr8PIsu7YDhQLqs0Ux3tLEQnuJnK366PWm8T0WS/RnIV/LnDHHFmrF\n" +
		"MujYpE3gpy7CZVg6W7Rft6GMX2/zcpuWTwtP216XuUdIIOLtqqZPELqu3br3LuNo\n" +
		"bFeMJyLbcZPSHrMi1Mf8586AG3sncim9vEubm04GpL9jlv6M1quov7n9D4fBvl6b\n" +
		"Zo6UyB3Z+E3/yGXvpA4pweQaGsv1lGF6GaheDvCZUJ8cXMbXoyEnflKZedtdoNmD\n" +
		"gief5gvH0dYpi3kUtRj8nPhgI2ur3Op3j2K/J73j8+mRozmR3sURqHCM16LZXIs8\n" +
		"LFq6BhM2AEl8rrVdRnqTunnyzcui6hqZarIgGf09dI/skg2fS74tn/WxgQ+FJxGE\n" +
		"k+vfv68L+NeJ+YsbWz35dyJUEmzsleLZ5/IhyXOH3A5uIkyrFQ/ThwflQ8bwi9lC\n" +
		"UGKK+PMGm2QZyI+Y5qrK5cdiUyM5liqSswEBhg0AzwZO4UVrlTegFCyI0nSGfSbH\n" +
		"dr8SLgPOqVp+HGZ6yDIysgEdAm+4/3wu3h5xAgMBAAGjUzBRMB0GA1UdDgQWBBS7\n" +
		"RsklNjFgYdXQAFA96q1GBTlv0DAfBgNVHSMEGDAWgBS7RsklNjFgYdXQAFA96q1G\n" +
		"BTlv0DAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4ICAQDFH0wH2M9G\n" +
		"UbvUdhji0gUm2FxvD0L8RmeRzMo5EihUm2lGBoHcpzaC4TNMHcmD/V7Q+W+80X0j\n" +
		"bKxJ6mLUOez5hznwSefqyrrOBdYGCZaLUiaYFFrccvZ12iyy0hk9FPE3z1u5H9U2\n" +
		"9IqtrmfBYWEG2zWTMpSQ9GipVBfFfP4K3wM7Lap+hdcp2LSwaqWsFZe+LyVZHxM8\n" +
		"u4cjyPSjUztaZl+KS4n0MryoboL6MbWgPOqvnotoHzYe4psphzoC1xnHbaNjETMR\n" +
		"Kk01+j1KMl9hhLn2K/DRFNJpRoGPJdaywND1hybF0qGQtIC8LiLYIkVrlzy2c7oo\n" +
		"Q56Jm7mkrG/pFD2eQyoyLgWt2ggmdhYqpYYSei0j+TZEmNF2OpbJdfCNkBX2ndiv\n" +
		"uM5iXGbINfRgJuECop1rv26YFw+gonF6TIKHhUT0yfFbDEUnqgCHO9xqTdow+XTh\n" +
		"cH9bofXnB8RkeiFnlyw6G+zthff753tuf1LsSJ1xxSh3J1rPJsrEapH6oOIkatva\n" +
		"cgv28ZBz0I0lr6k/ZWvnoNyH9+nxmoIvq5vKjdLuZtMS18JJ5II9ngM8Q2pj/3YZ\n" +
		"ls8T29AUCm62XMdL0/sBcOzhvFSCZBPcQrmcmA/JmR+4UE6YzXK5xAC47bkpZ/5M\n" +
		"iH9l5NcZRNmbCcw4s4Qn9MXY2AGOFFNEew==\n" +
		"-----END CERTIFICATE-----\n"

	const token string =
		"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdXRoX3JvbGUiOiJ1c2VyX3JvbGVfMCIsImlhdCI" +
		"6MTYyMzQxODMwMCwiaXNzIjoiemVuaXRlbF9hdXRoX3dhbXAiLCJqdGkiOiJ1c2VyXzAifQ.OkVZ3YO5" +
		"Ha4wgSWt0HcSBaqVap4ujPdnqwFP-JGYl2qtpykzV7pZCClQJPovMU7Y8LmXkjotwaEzOHrSF09ETenT" +
		"jIYvRBLZn-ljYz4H4a_Gab1OTakIlYmvMEfDGzDbXo3fkgd3GnZGbxVYzLCaBX7eBP6qsUwNBqO_c8X1" +
		"gqS04MI7BQKdcN3djAoIhgykb02mZJolJB46NcEJxx_UqXsCW6xZPlZyYLXug2q-fpMRo9LCcST8hfOS" +
		"U7v1ecG1ha504CaoJ5DchP5_AznvMtvkVSKr6RWYEQNcVUBDQvW1XIOc_wBdZdRFDxTt0OcH4-EeCUmA" +
		"DfQU9pjfH1NpNgOvAnLwP6OaOx5VFf1qEKiTKYhQIprobesl8odrTuTmbjdxJDRI4-NcFtTeoV6SvuR6" +
		"_y0shXZZrX9gLbQQhFBmNhkeDsdE-WcJ6Zr6uQC6d5y7T4bgi14Ow05d4gMIXEM8JZwu8ufuve-y-qtG" +
		"t0tvVwP2pgs-HCkQKZKOq4hA8N-bTrwknCOyCKzn6iXlHsRmTtq0SEwbAH7cqFXZR3n9kjcCv-3vIJju" +
		"CL7pt0IfcfU_PB5dlnvr_f4nfKITt8poiPGlizLxXXzgwU66OFk9xlbVW9qoloVN5ng6RIXYqxuB-LYD" +
		"uvSJlTNAmIZQeJqH8eNC7Wrv518v0e_tbZU"

	app = nil
	MiddlewareStack = nil
	bearer := "Bearer " + token
	os.Setenv("PREST_JWT_DEFAULT", "true")
	os.Setenv("PREST_DEBUG", "false")
	os.Setenv("PREST_JWT_KEY", cert)
	os.Setenv("PREST_JWT_ALGO", "RS256")

	config.Load()
	nd := appTestWithJwt()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, but got %d", respd.StatusCode)
	}
}

func TestJWTSignatureKo(t *testing.T) {
	app = nil
	bearer := "Bearer: eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImpvaG4uZG9lQHNvbWV3aGVyZS5jb20iLCJleHAiOjE1MjUzMzk2MTYsImlhdCI6MTUxNzU2MzYxNiwiaXNzIjoicHJpdmF0ZSIsImp0aSI6ImNlZmE3NGZlLTg5NGMtZmY2My1kODE2LTQ2MjBiOGNkOTJlZSIsIm9yZyI6InByaXZhdGUiLCJzdWIiOiJqb2huLmRvZSJ9.zGP1Xths2bK2r9FN0Gv1SzyoisO0dhRwvqrPvunGxUyU5TbkfdnTcQRJNYZzJfGILeQ9r3tbuakWm-NIoDlbbA"
	os.Setenv("PREST_JWT_DEFAULT", "true")
	os.Setenv("PREST_DEBUG", "false")
	os.Setenv("PREST_JWT_KEY", "s3cr3t")
	os.Setenv("PREST_JWT_ALGO", "HS256")
	config.Load()
	nd := appTestWithJwt()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status code 401, but got %d", respd.StatusCode)
	}
}

func appTest() *negroni.Negroni {
	n := GetApp()
	r := router.Get()
	if !config.PrestConf.Debug && !config.PrestConf.EnableDefaultJWT {
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

func appTestWithJwt() *negroni.Negroni {
	n := GetApp()
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")

	n.UseHandler(r)
	return n
}

func TestCors(t *testing.T) {
	MiddlewareStack = []negroni.Handler{}
	os.Setenv("PREST_DEBUG", "true")
	os.Setenv("PREST_CORS_ALLOWORIGIN", "*")
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	config.Load()
	app = nil
	r := router.Get()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	req, err := http.NewRequest("OPTIONS", server.URL, nil)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	req.Header.Set("Access-Control-Request-Method", "GET")
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected allow origin *, but got %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	methods := resp.Header.Get("Access-Control-Allow-Methods")
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		if !strings.Contains(methods, method) {
			t.Errorf("do not contain %s", method)
		}
	}
	if resp.Request.Method != "OPTIONS" {
		t.Errorf("expected method OPTIONS, but got %v", resp.Request.Method)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected HTTP status code 200, but got %v", resp.StatusCode)
	}
	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	if len(body) != 0 {
		t.Error("body is not empty")
	}
}
