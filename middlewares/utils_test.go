package middlewares

import (
	"net/http"
	"testing"

	"github.com/prest/prest/config"
	"github.com/prest/prest/middlewares/statements"
)

func Test_getVars(t *testing.T) {
	paths := getVars("foo/bar")
	if paths != nil {
		t.Errorf("expected nil, got %s", paths)
	}
}

func Test_permissionByMethod(t *testing.T) {
	permission := permissionByMethod("GET")
	if permission != statements.READ {
		t.Errorf("expected %x, got :%x", statements.READ, permission)
	}

	permission = permissionByMethod("POST")
	if permission != statements.WRITE {
		t.Errorf("expected %x, got :%x", statements.WRITE, permission)
	}

	permission = permissionByMethod("PATCH")
	if permission != statements.WRITE {
		t.Errorf("expected %x, got :%x", statements.WRITE, permission)
	}

	permission = permissionByMethod("PUT")
	if permission != statements.WRITE {
		t.Errorf("expected %x, got :%x", statements.WRITE, permission)
	}

	permission = permissionByMethod("DELETE")
	if permission != statements.DELETE {
		t.Errorf("expected %x, got :%x", statements.DELETE, permission)
	}

	permission = permissionByMethod("OPTION")
	if permission != "" {
		t.Errorf("expected to be empty, got :%x", permission)
	}
}

func Test_checkCors(t *testing.T) {
	allowed := checkCors(&http.Request{Method: http.MethodPost}, []string{"*"})
	if !allowed {
		t.Error("expected true, got false")
	}

	allowed = checkCors(&http.Request{Method: http.MethodHead}, []string{"*"})
	if allowed {
		t.Error("expected false, got true")
	}
}

func TestMatchURL(t *testing.T) {
	test := []struct {
		Label        string
		URL          string
		JWTWhiteList string
		match        bool
	}{
		{
			Label:        "auth",
			URL:          "/auth",
			JWTWhiteList: `\/auth`,
			match:        true,
		},
		{
			Label:        "auth regex",
			URL:          "/auth/any",
			JWTWhiteList: `\/auth\/.*`,
			match:        true,
		},
		{
			Label:        "auth2 lock",
			URL:          "/auth2",
			JWTWhiteList: `\/auth`,
			match:        true,
		},
		{
			Label:        "multi allow",
			URL:          "/auth",
			JWTWhiteList: `\/auth \/databases`,
			match:        true,
		},
	}

	for _, tt := range test {
		t.Run(tt.Label, func(t *testing.T) {
			config.PrestConf.JWTWhiteList = tt.JWTWhiteList
			match, err := MatchURL(tt.URL)
			if err != nil {
				t.Error(err)
			}
			if match != tt.match {
				t.Errorf("expected %v, but got %v\n", tt.match, match)
			}
		})
	}
}
