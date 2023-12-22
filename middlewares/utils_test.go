package middlewares

import (
	"net/http"
	"testing"

	"github.com/prest/prest/middlewares/statements"
	"github.com/stretchr/testify/require"
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
		JWTWhiteList []string
		match        bool
	}{
		{
			Label:        "auth",
			URL:          "/auth",
			JWTWhiteList: []string{`\/auth`},
			match:        true,
		},
		{
			Label:        "auth regex",
			URL:          "/auth/any",
			JWTWhiteList: []string{`\/auth\/.*`},
			match:        true,
		},
		{
			Label:        "auth2 lock",
			URL:          "/auth2",
			JWTWhiteList: []string{`\/auth`},
			match:        true,
		},
		{
			Label:        "multi allow",
			URL:          "/auth",
			JWTWhiteList: []string{`\/auth`, `\/databases`},
			match:        true,
		},
		{
			Label:        "multi allow, without endpoint escaping",
			URL:          "/auth",
			JWTWhiteList: []string{"/auth", "/databases"},
			match:        true,
		},
	}

	for _, tt := range test {
		tt := tt
		t.Run(tt.Label, func(t *testing.T) {
			match, err := MatchURL(tt.JWTWhiteList, tt.URL)
			require.NoError(t, err)
			require.Equal(t, tt.match, match)
		})
	}
}
