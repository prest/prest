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

	var testCases = []struct {
		description string
		database    string
	}{
		{"GET rows from primary cluster database prest-test returns OK", "prest-test"},
		{"GET rows from secondary cluster database secondary-db returns OK", "secondary-db"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		url := fmt.Sprintf("%s/%s/public/test", base, tc.database)
		testutils.DoRequest(t, url, nil, "GET", http.StatusOK, tc.description)
	}
}
