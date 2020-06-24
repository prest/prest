package controllers

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"

	"net/http/httptest"

	"bytes"
	"encoding/json"
)

func validate(t *testing.T, w *httptest.ResponseRecorder, r *http.Request, h http.HandlerFunc, where string) {
	fmt.Println("Test:", where)
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
	fmt.Println("Test:", where)
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
	fmt.Println("Test:", where)
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
	fmt.Println("Test:", where)
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
	fmt.Println("Test:", where)
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
	fmt.Println("Test:", where)
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
	fmt.Println("Test:", where)
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

	_, err = os.Create(fmt.Sprint(base, "/fulltable/funcs.read.sql"))
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

	write("SELECT * FROM test7 WHERE name = '{{defaultOrValue \"field1\" \"gopher\"}}'", "funcs.read.sql")
	write("SELECT * FROM test7 WHERE name = '{{.field1}}'", "get_all.read.sql")
	write("INSERT INTO test7 (name, surname) VALUES ('{{.field1}}', '{{.field2}}')", "write_all.write.sql")
	write("CREATE TABLE {{.field1}};", "create_table.write.sql")
	write("UPDATE test7 SET name = '{{.field1}}' WHERE surname = '{{.field2}}'", "patch_all.update.sql")
	write("UPDATE test7 SET surname = '{{.field1}}' WHERE name = '{{.field2}}'", "put_all.update.sql")
	write("DELETE FROM test7 WHERE name = '{{.field1}}'", "delete_all.delete.sql")
}

func removeMockScripts(base string) {
	log.Println("Remove scripts on", base)
	err := os.RemoveAll(base)
	if err != nil {
		log.Println(err)
	}
}
