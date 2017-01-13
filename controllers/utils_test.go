package controllers

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"net/http/httptest"

	"bytes"
	"encoding/json"

	"github.com/nuveo/prest/api"
	. "github.com/smartystreets/goconvey/convey"
)

func validate(w *httptest.ResponseRecorder, r *http.Request, h http.HandlerFunc, where string) {
	h(w, r)
	fmt.Println("Test:", where)
	So(w.Code, ShouldEqual, 200)
	_, err := ioutil.ReadAll(w.Body)
	So(err, ShouldBeNil)
}

func doValidGetRequest(url string, where string) {
	fmt.Println("Test:", where)
	resp, err := http.Get(url)
	So(err, ShouldBeNil)
	So(resp.StatusCode, ShouldEqual, 200)
	_, err = ioutil.ReadAll(resp.Body)
	So(err, ShouldBeNil)
}

func doValidPostRequest(url string, r api.Request, where string) {
	fmt.Println("Test:", where)
	byt, err := json.Marshal(r)
	So(err, ShouldBeNil)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(byt))
	So(err, ShouldBeNil)
	So(resp.StatusCode, ShouldEqual, 200)
	_, err = ioutil.ReadAll(resp.Body)
	So(err, ShouldBeNil)
}

func doValidDeleteRequest(url string, where string) {
	fmt.Println("Test:", where)
	req, err := http.NewRequest("DELETE", url, nil)
	So(err, ShouldBeNil)
	client := &http.Client{}
	resp, err := client.Do(req)
	So(err, ShouldBeNil)
	So(resp.StatusCode, ShouldEqual, 200)
	_, err = ioutil.ReadAll(resp.Body)
	So(err, ShouldBeNil)
}

func doValidPutRequest(url string, r api.Request, where string) {
	fmt.Println("Test:", where)
	byt, err := json.Marshal(r)
	So(err, ShouldBeNil)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(byt))
	So(err, ShouldBeNil)
	client := &http.Client{}
	resp, err := client.Do(req)
	So(err, ShouldBeNil)
	So(resp.StatusCode, ShouldEqual, 200)
	_, err = ioutil.ReadAll(resp.Body)
	So(err, ShouldBeNil)
}

func doValidPatchRequest(url string, r api.Request, where string) {
	fmt.Println("Test:", where)
	byt, err := json.Marshal(r)
	So(err, ShouldBeNil)
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(byt))
	So(err, ShouldBeNil)
	client := &http.Client{}
	resp, err := client.Do(req)
	So(err, ShouldBeNil)
	So(resp.StatusCode, ShouldEqual, 200)
	_, err = ioutil.ReadAll(resp.Body)
	So(err, ShouldBeNil)
}

func doRequest(url string, r api.Request, method string, expectedStatus int, where string) {
	fmt.Println("Test:", where)
	var byt []byte
	var err error

	if r.Data != nil {
		byt, err = json.Marshal(r)
		So(err, ShouldBeNil)

	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(byt))
	So(err, ShouldBeNil)

	client := &http.Client{}
	resp, err := client.Do(req)

	So(err, ShouldBeNil)
	So(resp.StatusCode, ShouldEqual, expectedStatus)

	_, err = ioutil.ReadAll(resp.Body)
	So(err, ShouldBeNil)

}
