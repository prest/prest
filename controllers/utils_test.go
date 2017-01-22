package controllers

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

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

func createMockScripts(base string) {
	log.Println("Create scripts on", base)
	err := os.Mkdir(fmt.Sprint(base, "/fulltable"), 0777)
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/get_all.read.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/write_all.write.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/create_table.write.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/patch_all.update.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/put_all.update.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/delete_all.delete.sql"))
	if err != nil {
		log.Println(err)
	}
}

func writeMockScripts(base string) {
	base = fmt.Sprint(base, "/fulltable/")
	log.Println("Write scripts on", base)

	write := func(sql, fileName string) {
		var file, err = os.OpenFile(fmt.Sprint(base, fileName), os.O_RDWR, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// write some text to file
		_, err = file.WriteString(sql)
		if err != nil {
			log.Fatal(err)
		}
		// save changes
		err = file.Sync()
		if err != nil {
			log.Fatal(err)
		}
	}

	write("SELECT * FROM test7 WHERE name = {{.Field1}}", "get_all.read.sql")
	write("INSERT INTO test7 (name, surname) VALUES ({{.Field1}}, {{.Field2}}) RETURNING id", "write_all.write.sql")
	write("CREATE TABLE {{.Field1}};", "create_table.write.sql")
	write("UPDATE test7 SET name = {{.Field1}} WHERE surname = {{.Field2}}", "patch_all.update.sql")
	write("UPDATE test7 SET surname = {{.Field1}} WHERE name = {{.Field2}}", "put_all.update.sql")
	write("DELETE FROM test7 WHERE name = {{.Field1}}", "delete_all.delete.sql")
}

func removeMockScripts(base string) {
	log.Println("Remove scripts on", base)
	err := os.RemoveAll(base)
	if err != nil {
		log.Println(err)
	}
}
