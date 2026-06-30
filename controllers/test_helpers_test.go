package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
	pctx "github.com/prest/prest/v2/context"
)

func withTestTimeout(ctx context.Context) context.Context {
	return context.WithValue(ctx, pctx.HTTPTimeoutKey, 60) //nolint:staticcheck
}

func crudRequest(method, path string, vars map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req = mux.SetURLVars(req, vars)
	return req.WithContext(withTestTimeout(req.Context()))
}

type recordingCacher struct {
	key   string
	value string
}

func (c *recordingCacher) BuntSet(key, value string) {
	c.key = key
	c.value = value
}
