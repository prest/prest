package middlewares

import (
	"github.com/palevi67/prest/middlewares/statements"
	"net/http"
	"testing"
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
