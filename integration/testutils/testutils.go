package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// DoRequest function used to test internal http requests
func DoRequest(
	t *testing.T,
	url string,
	r interface{},
	method string,
	expectedStatus int,
	where string,
	expectedBody ...string,
) {
	DoRequestWithHeaders(
		t, url, r, method, expectedStatus, where, nil, expectedBody...)
}

// DoRequestWithHeaders sends an HTTP request with optional extra headers.
// If the expectedStatus is 0, the request is expected to fail.
// If the expectedStatus is not 0, the request is expected to succeed and the response body is expected to be in the expectedBody slice.
// If the expectedBody is provided, the request is expected to return the body in the expectedBody slice.
func DoRequestWithHeaders(
	t *testing.T,
	url string,
	r interface{},
	method string,
	expectedStatus int,
	where string,
	headers map[string]string,
	expectedBody ...string,
) {
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
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.Nil(t, err, "error on Do Request")
	if err != nil {
		fmt.Printf("error %+v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	assert.Nil(t, err, "error on io ReadAll")
	if err != nil {
		fmt.Printf("error %+v", err)
		return
	}

	bodyStr := string(body)
	fmt.Printf("test: %s body: %s\n", t.Name(), bodyStr)
	assert.Equal(t, expectedStatus, resp.StatusCode)

	if len(expectedBody) > 0 {
		for _, expected := range expectedBody {
			assert.True(t,
				strings.Contains(bodyStr, expected),
				fmt.Sprintf("expected %s not found in body %s", expected, bodyStr))
		}
	}
}

// DoRequestRaw sends a request with a raw body and optional headers.
func DoRequestRaw(t *testing.T, url string, body []byte, method string, expectedStatus int, where string, headers map[string]string, expectedBody ...string) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	assert.Nil(t, err, "error on New Request")
	if err != nil {
		fmt.Printf("error %+v", err)
		return
	}

	req.Header.Add("X-Application", "prest")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.Nil(t, err, "error on Do Request")
	if err != nil {
		fmt.Printf("error %+v", err)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	assert.Nil(t, err, "error on io ReadAll")
	if err != nil {
		fmt.Printf("error %+v", err)
		return
	}

	bodyStr := string(respBody)
	fmt.Printf("test: %s body: %s\n", t.Name(), bodyStr)
	assert.Equal(t, expectedStatus, resp.StatusCode)

	if len(expectedBody) > 0 {
		for _, expected := range expectedBody {
			assert.True(t,
				strings.Contains(bodyStr, expected),
				fmt.Sprintf("expected %s not found in body %s", expected, bodyStr))
		}
	}
}
