package middlewares

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"

	"github.com/prest/prest/v2/middlewares/statements"

	"github.com/clbanning/mxj/j2x"
)

var (
	ErrXMLBadRequest = `<?xml version="1.0" encoding="utf-8"?>
<errors xmlns="http://schemas.google.com/g/2005">
  <error>
    <reason>internal</reason>
    <internalReason>%s</internalReason>
  </error>
  <code>400</code> 
</errors>`
)

// getVars extracts {database}/{schema}/{table} from a request path. A real
// http.Request.URL.Path always starts with "/", so a 3-segment resource route
// splits into 4 elements (leading "") and a "/batch/..." route (one extra
// literal segment) splits into 5. Both prefixes are stripped explicitly here
// rather than inferred from segment count alone — that used to make any
// 4-element split fall through to "not a table path" (nil, which
// AccessControl treats as unenforced), silently skipping TablePermissions on
// /batch/{database}/{schema}/{table}.
func getVars(path string) (paths map[string]string) {
	segments := strings.Split(path, "/")
	if len(segments) > 0 && segments[0] == "" {
		segments = segments[1:]
	}
	if len(segments) == 4 && segments[0] == "batch" {
		segments = segments[1:]
	}
	if len(segments) != 3 {
		return nil
	}
	return map[string]string{
		"database": segments[0],
		"schema":   segments[1],
		"table":    segments[2],
	}
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
	byt, _ := io.ReadAll(recorder.Body)
	if recorder.Code >= 400 {
		trimmed := strings.TrimSpace(string(byt))
		// Pass through bodies that are already JSON (e.g. MCP JSON-RPC errors,
		// controller {"error":"..."} payloads) to avoid double-wrapping.
		if json.Valid([]byte(trimmed)) {
			byt = []byte(trimmed)
		} else {
			m := map[string]string{"error": trimmed}
			byt, _ = json.MarshalIndent(m, "", "\t")
		}
	}
	switch format {
	case "xml":
		xmldata, err := j2x.JsonToXml(byt)
		if err != nil {
			http.Error(w, fmt.Sprintf(ErrXMLBadRequest, err.Error()), http.StatusBadRequest)
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

// MatchURL matches the given url with a whitelist.
func MatchURL(url string, whitelist []string) (match bool, err error) {
	for _, exp := range whitelist {
		match, err = regexp.Match(exp, []byte(url))
		if match || err != nil {
			return
		}
	}
	return
}
