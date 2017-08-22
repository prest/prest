package middlewares

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/clbanning/mxj/j2x"
	"github.com/prest/statements"
)

func getVars(path string) (paths map[string]string) {
	pathList := strings.Split(path, "/")

	if len(pathList) < 3 || len(pathList) > 4 {
		return nil
	} else if len(pathList) == 4 {
		pathList = pathList[1:]
	}
	paths = make(map[string]string, 0)
	paths["database"] = pathList[0]
	paths["schema"] = pathList[1]
	paths["table"] = pathList[2]

	return
}

func permissionByMethod(method string) (permission string) {
	switch method {
	case "GET":
		permission = statements.READ
	case "POST", "PATCH", "PUT":
		permission = statements.WRITE
	case "DELETE":
		permission = statements.DELETE
	default:
		permission = ""
	}

	return
}

func renderFormat(w http.ResponseWriter, recorder *httptest.ResponseRecorder, format string) {
	for key := range recorder.Header() {
		w.Header().Set(key, recorder.Header().Get(key))
	}
	byt, _ := ioutil.ReadAll(recorder.Body)
	if recorder.Code >= 400 {
		m := make(map[string]string)
		m["error"] = strings.TrimSpace(string(byt))
		byt, _ = json.MarshalIndent(m, "", "\t")
	}
	switch format {
	case "xml":
		xmldata, err := j2x.JsonToXml(byt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		xmlStr := fmt.Sprintf("<objects>%s</objects>", string(xmldata))
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(recorder.Code)
		w.Write([]byte(xmlStr))
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(recorder.Code)
		w.Write(byt)
	}
}

var defaultAllowMethods = []string{
	"GET",
	"POST",
	"PUT",
	"PATCH",
	"DELETE",
	"OPTIONS",
}

const (
	headerAllowOrigin      = "Access-Control-Allow-Origin"
	headerAllowCredentials = "Access-Control-Allow-Credentials"
	headerAllowHeaders     = "Access-Control-Allow-Headers"
	headerAllowMethods     = "Access-Control-Allow-Methods"
	headerOrigin           = "Origin"
)

func checkCors(r *http.Request, origin []string) (allowed bool) {
	var mAllowed bool
	for _, m := range defaultAllowMethods {
		if m == r.Method {
			mAllowed = true
			break
		}
	}
	if !mAllowed {
		return
	}
	org := r.Header.Get(headerOrigin)
	var oAllowed bool
	for _, o := range origin {
		if o == org || o == "*" || org == "*" {
			oAllowed = true
			break
		}
	}
	if oAllowed && mAllowed {
		allowed = true
	}
	return
}
