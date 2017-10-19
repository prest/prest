package log

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"
)

func getOutput(logFunc func(msg ...interface{}), msg ...interface{}) ([]byte, error) {
	rescueStdout := os.Stdout
	defer func() { os.Stdout = rescueStdout }()

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	os.Stdout = w

	logFunc(msg...)

	err = w.Close()
	if err != nil {
		return nil, err
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func validate(key string, logFunc func(msg ...interface{}), valueExpected string, msg ...interface{}) (err error) {
	out, err := getOutput(logFunc, msg...)
	if err != nil {
		return
	}
	match, err := regexp.Match(valueExpected, out)
	if err != nil {
		return
	} else if !match {
		err = fmt.Errorf("Error, '%s' printed %q, expected %q", key, string(out), valueExpected)
	}
	return
}

func TestLog(t *testing.T) {
	now = func() time.Time { return time.Unix(1498405744, 0) }
	DebugMode = false

	data := []struct {
		key           string
		logFunc       func(msg ...interface{})
		expectedValue string
	}{
		{"Println", Println, "\x1b\\[37m2017/06/25 15:49:04 \\[msg\\] log test\x1b\\[0;00m\n"},
		{"Errorln", Errorln, "\x1b\\[91m2017/06/25 15:49:04 \\[error\\] log test\x1b\\[0;00m\n"},
		{"Warningln", Warningln, "\x1b\\[93m2017/06/25 15:49:04 \\[warning\\] log test\x1b\\[0;00m\n"},
		{"Debugln", Debugln, ""},
	}
	formattedData := []struct {
		key           string
		logFunc       func(msg ...interface{})
		expectedValue string
	}{
		{"Printf", Printf, "\x1b\\[37m2017/06/25 15:49:04 \\[msg\\] formatted log 1.12\x1b\\[0;00m"},
		{"Errorf", Errorf, "\x1b\\[91m2017/06/25 15:49:04 \\[error\\] formatted log 1.12\x1b\\[0;00m"},
		{"Warningf", Warningf, "\x1b\\[93m2017/06/25 15:49:04 \\[warning\\] formatted log 1.12\x1b\\[0;00m"},
		{"Debugf", Debugf, ""},
	}
	for _, v := range data {
		err := validate(v.key, v.logFunc, v.expectedValue, "log test")
		if err != nil {
			t.Fatal(err.Error())
		}
	}
	for _, v := range formattedData {
		err := validate(v.key, v.logFunc, v.expectedValue, "%s %s %.2f", "formatted", "log", 1.1234)
		if err != nil {
			t.Fatal(err.Error())
		}
	}
	DebugMode = true

	err := validate("Debugln", Debugln, "\x1b\\[96m2017/06/25 15:49:04 \\[debug\\] log_test.go:\\d+ log test\x1b\\[0;00m\n", "log test")
	if err != nil {
		t.Fatal(err.Error())
	}
	err = validate("Debugf", Debugf, "\x1b\\[96m2017/06/25 15:49:04 \\[debug\\] log_test.go:\\d+ formatted log 1.12\x1b\\[0;00m", "%s %s %.2f", "formatted", "log", 1.1234)
	if err != nil {
		t.Fatal(err.Error())
	}

}

func TestHTTPError(t *testing.T) {
	now = func() time.Time { return time.Unix(1498405744, 0) }

	rescueStdout := os.Stdout
	DebugMode = false
	defer func() { os.Stdout = rescueStdout }()

	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	os.Stdout = w

	handler := func(w http.ResponseWriter, r *http.Request) {
		HTTPError(w, http.StatusBadRequest)
	}

	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	httpw := httptest.NewRecorder()
	handler(httpw, req)

	resp := httpw.Result()
	body, _ := ioutil.ReadAll(resp.Body)

	os.Stdout = rescueStdout
	err = w.Close()
	if err != nil {
		return
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		return
	}

	valueExpected := "\x1b[91m2017/06/25 15:49:04 [error] Bad Request\x1b[0;00m\n"
	if string(out) != valueExpected {
		t.Fatalf("Error, 'HTTPError' printed %q, expected %q", string(out), valueExpected)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Error, 'HTTPError' status code %v, expected 400", resp.StatusCode)
	}

	valueExpected = "{\n\t\"error\": \"Bad Request\",\n\t\"status\": \"error\"\n}\n"
	if string(body) != valueExpected {
		t.Fatalf("Error, 'HTTPError' write to client %q, expected %q", string(body), valueExpected)
	}

}

func TestMaxLineSize(t *testing.T) {
	now = func() time.Time { return time.Unix(1498405744, 0) }
	DebugMode = false

	MaxLineSize = 30
	out, err := getOutput(Printf, "0123456789012345678901234567890123456789")
	if err != nil {
		t.Fatal(err.Error())
	}

	expectedValue := []byte("\x1b[37m2017/06/25 15:49:04 [msg]...")
	if !bytes.Equal(out, expectedValue) {
		t.Fatalf("Error, printed %q, expected %q", string(out), expectedValue)
	}

	out, err = getOutput(Println, "0123456789012345678901234567890123456789")
	if err != nil {
		t.Fatal(err.Error())
	}

	expectedValue = []byte("\x1b[37m2017/06/25 15:49:04 [msg]...\n")
	if !bytes.Equal(out, expectedValue) {
		t.Fatalf("Error, printed %q, expected %q", string(out), expectedValue)
	}
}
