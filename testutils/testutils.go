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
