package testutils

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
)

// DoRequest function used to test internal http requests
func DoRequest(t *testing.T, url string, r interface{}, method string, expectedStatus int, where string, expectedBody ...string) {
	var byt []byte
	var err error

	if r != nil {
		byt, err = json.Marshal(r)
		if err != nil {
			t.Error("error on json marshal", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(byt))
	if err != nil {
		t.Error("error on New Request", err)
	}

	req.Header.Add("X-Application", "prest")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Error("error on Do Request", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Errorf("%s expected %d, got: %d", url, expectedStatus, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("error on ioutil ReadAll", err)
	}

	if len(expectedBody) > 0 {
		if !containsStringInSlice(expectedBody, string(body)) {
			t.Errorf("expected %q, got: %q", expectedBody, string(body))
		}
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
