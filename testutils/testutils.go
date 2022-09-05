package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// DoRequest function used to test internal http requests
func DoRequest(t *testing.T, url string, r interface{}, method string, expectedStatus int, where string, expectedBody ...string) {
	var byt []byte
	var err error

	if r != nil {
		byt, err = json.Marshal(r)
		require.Nil(t, err, "error on json marshal")
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(byt))
	require.Nil(t, err, "error on New Request")

	req.Header.Add("X-Application", "prest")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.Nil(t, err, "error on Do Request")

	body, err := io.ReadAll(resp.Body)
	require.Nil(t, err, "error on io ReadAll")

	fmt.Printf("test: %s body: %s\n", t.Name(), string(body))
	require.Equal(t, expectedStatus, resp.StatusCode)

	if len(expectedBody) > 0 {
		require.True(t, containsStringInSlice(expectedBody, string(body)),
			fmt.Sprintf("expected %q, got: %q", expectedBody, string(body)))
	}
}

// containsStringInSlice check if there is string in slice
func containsStringInSlice(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
