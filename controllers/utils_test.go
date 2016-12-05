package controllers

import (
	"io/ioutil"
	"net/http"

	"net/http/httptest"

	"bytes"
	"encoding/json"

	"github.com/nuveo/prest/api"
	. "github.com/smartystreets/goconvey/convey"
)

func validate(w *httptest.ResponseRecorder, r *http.Request, h http.HandlerFunc) {
	h(w, r)
	So(w.Code, ShouldEqual, 200)
	_, err := ioutil.ReadAll(w.Body)
	So(err, ShouldBeNil)
}

func doValidGetRequest(url string) {
	resp, err := http.Get(url)
	So(err, ShouldBeNil)
	So(resp.StatusCode, ShouldEqual, 200)
	_, err = ioutil.ReadAll(resp.Body)
	So(err, ShouldBeNil)
}

func doValidPostRequest(url string, r api.Request) {
	byt, err := json.Marshal(r)
	So(err, ShouldBeNil)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(byt))
	So(err, ShouldBeNil)
	So(resp.StatusCode, ShouldEqual, 200)
	_, err = ioutil.ReadAll(resp.Body)
	So(err, ShouldBeNil)
}

func doValidDeleteRequest(url string) {
	req, err := http.NewRequest("DELETE", url, nil)
	So(err, ShouldBeNil)
	client := &http.Client{}
	resp, err := client.Do(req)
	So(err, ShouldBeNil)
	So(resp.StatusCode, ShouldEqual, 200)
	_, err = ioutil.ReadAll(resp.Body)
	So(err, ShouldBeNil)
}
