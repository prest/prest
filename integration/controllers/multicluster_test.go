package controllers_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestMultiClusterSelect(t *testing.T) {
	base := helpers.MultiClusterServerURL(t)

	for _, db := range helpers.Databases() {
		url := fmt.Sprintf("%s/%s/public/test", base, db)
		testutils.DoRequest(t, url, nil, "GET", http.StatusOK, "MultiClusterSelect")
	}
}
