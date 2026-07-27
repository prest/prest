package middlewares

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/middlewares/statements"
	"github.com/stretchr/testify/require"
)

func Test_getVars(t *testing.T) {
	t.Parallel()

	if got := getVars("foo/bar"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}

	got := getVars("prest/public/users")
	require.Equal(t, "prest", got["database"])
	require.Equal(t, "public", got["schema"])
	require.Equal(t, "users", got["table"])

	// A real http.Request.URL.Path always carries the leading slash.
	got = getVars("/prest/public/users")
	require.Equal(t, "prest", got["database"])
	require.Equal(t, "public", got["schema"])
	require.Equal(t, "users", got["table"])

	// /batch/{database}/{schema}/{table}: the shape that used to fall through
	// to nil (any 4-element split was treated as "drop the first element",
	// which only makes sense when that first element is the leading "").
	got = getVars("/batch/prest/public/users")
	require.Equal(t, "prest", got["database"])
	require.Equal(t, "public", got["schema"])
	require.Equal(t, "users", got["table"])

	// A genuinely malformed 4-segment path with neither a leading slash nor a
	// "batch" prefix has no valid interpretation and must be rejected, not
	// have an arbitrary segment dropped.
	require.Nil(t, getVars("prest/public/users/extra"))
	require.Nil(t, getVars("/prest/public/users/extra"))
}

func Test_permissionByMethod(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
		t.Run(tt.Label, func(t *testing.T) {
			match, err := MatchURL(tt.URL, tt.JWTWhiteList)
			if err != nil {
				t.Error(err)
			}
			if match != tt.match {
				t.Errorf("expected %v, but got %v\n", tt.match, match)
			}
		})
	}
}
