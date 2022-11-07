package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// DoRequest function used to test internal http requests
func DoRequest(t *testing.T, url string, r interface{}, method string, expectedStatus int, where string, expectedBody ...string) {
	var byt []byte
	var err error

	if r != nil {
		byt, err = json.Marshal(r)
		assert.Nil(t, err, "error on json marshal")
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(byt))
	assert.Nil(t, err, "error on New Request")
	if err != nil {
		fmt.Printf("error %+v", err)
		return
	}

	req.Header.Add("X-Application", "prest")

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.Nil(t, err, "error on Do Request")
	if err != nil {
		fmt.Printf("error %+v", err)
		return
	}

	body, err := io.ReadAll(resp.Body)
	assert.Nil(t, err, "error on io ReadAll")
	if err != nil {
		fmt.Printf("error %+v", err)
		return
	}

	fmt.Printf("test: %s body: %s\n", t.Name(), string(body))
	assert.Equal(t, expectedStatus, resp.StatusCode)

	if len(expectedBody) > 0 {
		assert.True(t, containsStringInSlice(expectedBody, string(body)),
			fmt.Sprintf("expected %q, got: %q", expectedBody[0], string(body)))
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
