// Copyright (c) 2012-2013 Jason McVetta.  This is Free Software, released
// under the terms of the GPL v3.  See http://www.gnu.org/copyleft/gpl.html for
// details.  Resist intellectual serfdom - the ownership of ideas is akin to
// slavery.

package napping

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}

func TestInvalidUrl(t *testing.T) {
	//
	//  Missing protocol scheme - url.Parse should fail
	//

	url := "://foobar.com"
	_, err := Get(url, nil, nil, nil)
	assert.NotEqual(t, nil, err)
	//
	// Unsupported protocol scheme - HttpClient.Do should fail
	//
	url = "foo://bar.com"
	_, err = Get(url, nil, nil, nil)
	assert.NotEqual(t, nil, err)
}

type structType struct {
	Foo int
	Bar string
}

type errorStruct struct {
	Status  int
	Message string
}

var (
	fooParams = Params{"foo": "bar"}
	barParams = Params{"bar": "baz"}
	fooStruct = structType{
		Foo: 111,
		Bar: "foo",
	}
	barStruct = structType{
		Foo: 222,
		Bar: "bar",
	}
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandleGet))
	defer srv.Close()
	//
	// Good request
	//
	url := "http://" + srv.Listener.Addr().String()
	p := fooParams.AsUrlValues()
	res := structType{}
	resp, err := Get(url, &p, &res, nil)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 200, resp.Status())
	assert.Equal(t, res, barStruct)
	//
	// Bad request
	//
	url = "http://" + srv.Listener.Addr().String()
	p = Params{"bad": "value"}.AsUrlValues()
	e := errorStruct{}
	resp, err = Get(url, &p, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status() == 200 {
		t.Error("Server returned 200 success when it should have failed")
	}
	assert.Equal(t, 500, resp.Status())
	expected := errorStruct{
		Message: "Bad query params: bad=value",
		Status:  500,
	}
	resp.Unmarshal(&e)
	assert.Equal(t, e, expected)
}

// TestDefaultParams tests using per-session default query parameters.
func TestDefaultParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandleGet))
	defer srv.Close()
	//
	// Good request
	//
	url := "http://" + srv.Listener.Addr().String()
	p := fooParams.AsUrlValues()
	res := structType{}
	s := Session{
		Params: &p,
	}
	resp, err := s.Get(url, nil, &res, nil)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 200, resp.Status())
	assert.Equal(t, res, barStruct)
	//
	// Bad request
	//
	url = "http://" + srv.Listener.Addr().String()
	p = Params{"bad": "value"}.AsUrlValues()
	e := errorStruct{}
	resp, err = Get(url, &p, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status() == 200 {
		t.Error("Server returned 200 success when it should have failed")
	}
	assert.Equal(t, 500, resp.Status())
	expected := errorStruct{
		Message: "Bad query params: bad=value",
		Status:  500,
	}
	resp.Unmarshal(&e)
	assert.Equal(t, e, expected)
}

func TestDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandleDelete))
	defer srv.Close()
	url := "http://" + srv.Listener.Addr().String()
	resp, err := Delete(url, nil, nil, nil)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 200, resp.Status())
}

func TestHead(t *testing.T) {
	// TODO: test result
	srv := httptest.NewServer(http.HandlerFunc(HandleHead))
	defer srv.Close()
	url := "http://" + srv.Listener.Addr().String()
	resp, err := Head(url, nil, nil)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 200, resp.Status())
}

func TestOptions(t *testing.T) {
	// TODO: test result
	srv := httptest.NewServer(http.HandlerFunc(HandleOptions))
	defer srv.Close()
	url := "http://" + srv.Listener.Addr().String()
	resp, err := Options(url, nil, nil)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 200, resp.Status())
}

func TestPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandlePost))
	defer srv.Close()
	s := Session{}
	s.Log = true
	url := "http://" + srv.Listener.Addr().String()
	payload := fooStruct
	res := structType{}
	resp, err := s.Post(url, &payload, &res, nil)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 200, resp.Status())
	assert.Equal(t, res, barStruct)
}

func TestPostUnmarshalable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandlePost))
	defer srv.Close()
	type ft func()
	var f ft
	url := "http://" + srv.Listener.Addr().String()
	res := structType{}
	payload := f
	_, err := Post(url, &payload, &res, nil)
	assert.NotEqual(t, nil, err)
	_, ok := err.(*json.UnsupportedTypeError)
	if !ok {
		t.Log(err)
		t.Error("Expected json.UnsupportedTypeError")
	}
}

func TestPostParamsInUrl(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandlePost))
	defer srv.Close()
	s := Session{}
	s.Log = true
	u := "http://" + srv.Listener.Addr().String()
	u += "?spam=eggs" // Add query params to URL
	payload := fooStruct
	res := structType{}
	resp, err := s.Post(u, &payload, &res, nil)
	if err != nil {
		t.Error(err)
	}
	expected := &url.Values{}
	expected.Add("spam", "eggs")
	assert.Equal(t, expected, resp.Params)
}

func TestPut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandlePut))
	defer srv.Close()
	url := "http://" + srv.Listener.Addr().String()
	res := structType{}
	resp, err := Put(url, &fooStruct, &res, nil)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, resp.Status(), 200)
	// Server should return NO data
	assert.Equal(t, resp.RawText(), "")
}

func TestPatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandlePatch))
	defer srv.Close()
	url := "http://" + srv.Listener.Addr().String()
	res := structType{}
	resp, err := Patch(url, &fooStruct, &res, nil)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, resp.Status(), 200)
	// Server should return NO data
	assert.Equal(t, resp.RawText(), "")
}

func TestRawRequestWithData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandleRaw))
	defer srv.Close()

	var payload = bytes.NewBufferString("napping")
	res := structType{}
	req := Request{
		Url:        "http://" + srv.Listener.Addr().String(),
		Method:     "PUT",
		RawPayload: true,
		Payload:    payload,
		Result:     &res,
	}

	resp, err := Send(&req)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, resp.Status(), 200)
	assert.Equal(t, res.Bar, "napping")
}

func TestRawRequestWithoutData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandleRaw))
	defer srv.Close()

	var payload *bytes.Buffer
	res := structType{}
	req := Request{
		Url:        "http://" + srv.Listener.Addr().String(),
		Method:     "PUT",
		RawPayload: true,
		Payload:    payload,
		Result:     &res,
	}

	resp, err := Send(&req)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, resp.Status(), 200)
	assert.Equal(t, res.Bar, "empty")
}

func TestRawRequestInvalidType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandleRaw))
	defer srv.Close()

	payload := structType{}
	res := structType{}
	req := Request{
		Url:        "http://" + srv.Listener.Addr().String(),
		Method:     "PUT",
		RawPayload: true,
		Payload:    payload,
		Result:     &res,
	}

	_, err := Send(&req)

	if err == nil {
		t.Error("Validation error expected")
	} else {
		assert.Equal(t, err.Error(), "Payload must be of type *bytes.Buffer if RawPayload is set to true")
	}
}

// TestRawResponse tests capturing the raw response body.
func TestRawResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(HandleRaw))
	defer srv.Close()

	var payload = bytes.NewBufferString("napping")
	req := Request{
		Url:                 "http://" + srv.Listener.Addr().String(),
		Method:              "PUT",
		RawPayload:          true,
		CaptureResponseBody: true,
		Payload:             payload,
	}

	resp, err := Send(&req)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, resp.Status(), 200)
	rawResponseStruct := structType{
		Foo: 0,
		Bar: "napping",
	}

	blob, _ := json.Marshal(rawResponseStruct)
	assert.Equal(t, bytes.Equal(resp.ResponseBody.Bytes(), blob), true)
}

func JSONError(w http.ResponseWriter, msg string, code int) {
	e := errorStruct{
		Status:  code,
		Message: msg,
	}
	blob, err := json.Marshal(e)
	if err != nil {
		http.Error(w, msg, code)
		return
	}
	http.Error(w, string(blob), code)
}

func HandleGet(w http.ResponseWriter, req *http.Request) {
	method := strings.ToUpper(req.Method)
	if method != "GET" {
		msg := fmt.Sprintf("Expected method GET, received %s", method)
		http.Error(w, msg, 500)
		return
	}
	u := req.URL
	q := u.Query()
	for k := range fooParams {
		if fooParams[k] != q.Get(k) {
			msg := "Bad query params: " + u.Query().Encode()
			JSONError(w, msg, http.StatusInternalServerError)
			return
		}
	}
	//
	// Generate response
	//
	blob, err := json.Marshal(barStruct)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Add("content-type", "application/json")
	w.Write(blob)
}

func HandleDelete(w http.ResponseWriter, req *http.Request) {
	method := strings.ToUpper(req.Method)
	if method != "DELETE" {
		msg := fmt.Sprintf("Expected method DELETE, received %s", method)
		http.Error(w, msg, 500)
		return
	}
}

func HandleHead(w http.ResponseWriter, req *http.Request) {
	method := strings.ToUpper(req.Method)
	if method != "HEAD" {
		msg := fmt.Sprintf("Expected method HEAD, received %s", method)
		http.Error(w, msg, 500)
		return
	}
}

func HandleOptions(w http.ResponseWriter, req *http.Request) {
	method := strings.ToUpper(req.Method)
	if method != "OPTIONS" {
		msg := fmt.Sprintf("Expected method OPTIONS, received %s", method)
		http.Error(w, msg, 500)
		return
	}
}

func HandlePost(w http.ResponseWriter, req *http.Request) {
	method := strings.ToUpper(req.Method)
	if method != "POST" {
		msg := fmt.Sprintf("Expected method POST, received %s", method)
		http.Error(w, msg, 500)
		return
	}
	//
	// Parse Payload
	//
	if req.ContentLength <= 0 {
		msg := "Content-Length must be greater than 0."
		JSONError(w, msg, http.StatusLengthRequired)
		return
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var s structType
	err = json.Unmarshal(body, &s)
	if err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s != fooStruct {
		msg := "Bad request body"
		JSONError(w, msg, http.StatusBadRequest)
		return
	}
	//
	// Compose Response
	//
	blob, err := json.Marshal(barStruct)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Add("content-type", "application/json")
	w.Write(blob)
}

func HandlePut(w http.ResponseWriter, req *http.Request) {
	method := strings.ToUpper(req.Method)
	if method != "PUT" {
		msg := fmt.Sprintf("Expected method PUT, received %s", method)
		http.Error(w, msg, 500)
		return
	}
	//
	// Parse Payload
	//
	if req.ContentLength <= 0 {
		msg := "Content-Length must be greater than 0."
		JSONError(w, msg, http.StatusLengthRequired)
		return
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var s structType
	err = json.Unmarshal(body, &s)
	if err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s != fooStruct {
		msg := "Bad request body"
		JSONError(w, msg, http.StatusBadRequest)
		return
	}
	return
}

func HandlePatch(w http.ResponseWriter, req *http.Request) {
	method := strings.ToUpper(req.Method)
	if method != "PATCH" {
		msg := fmt.Sprintf("Expected method PATCH, received %s", method)
		http.Error(w, msg, 500)
		return
	}
	//
	// Parse Payload
	//
	if req.ContentLength <= 0 {
		msg := "Content-Length must be greater than 0."
		JSONError(w, msg, http.StatusLengthRequired)
		return
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var s structType
	err = json.Unmarshal(body, &s)
	if err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s != fooStruct {
		msg := "Bad request body"
		JSONError(w, msg, http.StatusBadRequest)
		return
	}
	return
}

func HandleRaw(w http.ResponseWriter, req *http.Request) {
	var err error
	var result = structType{}
	if req.ContentLength <= 0 {
		result.Bar = "empty"
	} else {
		var body []byte
		body, err = ioutil.ReadAll(req.Body)
		if err == nil {
			result.Bar = string(body)
		}
	}

	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var blob []byte
	blob, err = json.Marshal(result)

	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("content-type", "application/json")
	w.Write(blob)

	return
}
