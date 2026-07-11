// nolint
package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
)

func TestQueriesACL(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	path := "/_QUERIES/fulltable/get_all?field1=gopher"
	writePath := "/_QUERIES/fulltable/write_all?field1=gopherzin&field2=pereira"

	helpers.DoAuthRequest(t, base+path, nil, http.MethodGet, "", http.StatusUnauthorized, "QueriesACLNoToken")

	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)
	helpers.DoAuthRequest(t, base+path, nil, http.MethodGet, token, http.StatusOK, "QueriesACLAllowedRead")
	helpers.DoAuthRequest(t, base+writePath, nil, http.MethodPost, token, http.StatusUnauthorized, "QueriesACLDeniedWrite")
}
