package controllers

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func validate(t *testing.T, w *httptest.ResponseRecorder, r *http.Request, h http.HandlerFunc, where string) {
	h(w, r)

	if w.Code != 200 {
		t.Errorf("expected 200, got: %d", w.Code)
	}

	_, err := ioutil.ReadAll(w.Body)
	if err != nil {
		t.Error("error on ioutil ReadAll", err)
	}
}

func doValidGetRequest(t *testing.T, url string, where string) {
	resp, err := http.Get(url)
	if err != nil {
		t.Error("expected no errors in Get")
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("expected no errors in ioutil ReadAll")
	}
}

func doValidPostRequest(t *testing.T, url string, r map[string]interface{}, where string) {
	byt, err := json.Marshal(r)
	if err != nil {
		t.Error("expected no errors in json marshal, but was!")
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(byt))
	if err != nil {
		t.Error("expected no errors in Post")
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("expected no errors in ioutil ReadAll")
	}
}

func doValidDeleteRequest(t *testing.T, url string, where string) {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		t.Error("expected no errors in NewRequest, but was!")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Error("expected no errors in Do Request, but was!")
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("expected no errors in ioutil ReadAll")
	}
}

func doValidPutRequest(t *testing.T, url string, r map[string]interface{}, where string) {
	byt, err := json.Marshal(r)
	if err != nil {
		t.Error("expected no errors in json marshal, but was!")
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(byt))
	if err != nil {
		t.Error("expected no errors in PUT")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Error("expected no errors in Do Request, but was!")
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("expected no errors in ioutil ReadAll")
	}
}

func doValidPatchRequest(t *testing.T, url string, r map[string]interface{}, where string) {
	byt, err := json.Marshal(r)
	if err != nil {
		t.Error("expected no errors in json marshal, but was!")
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(byt))
	if err != nil {
		t.Error("expected no errors in PATCH")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Error("expected no errors in Do Request, but was!")
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("expected no errors in ioutil ReadAll")
	}
}

func doRequest(t *testing.T, url string, r interface{}, method string, expectedStatus int, where string, expectedBody ...string) {
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
		t.Errorf("expected %d, got: %d", expectedStatus, resp.StatusCode)
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

func containsStringInSlice(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
