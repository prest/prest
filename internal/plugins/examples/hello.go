// nolint
// all plugins must have their package name as `main`
// each plugin is isolated at compile time
package main

import (
	"encoding/json"
	"net/http"
)

var (
	// HTTPVars route variables for the current request
	HTTPVars map[string]string
	// URLQuery parses RawQuery and returns the corresponding values
	URLQuery map[string][]string
)

// Response return structure of the get method
type Response struct {
	HTTPVars map[string]string   `json:"http_vars"`
	URLQuery map[string][]string `json:"url_query"`
	MSG      string              `json:"msg"`
}

// GETHelloHandler plugin
// function is invoked via [go language plugin](https://pkg.go.dev/plugin),
// it is not possible to pass parameters, that's why there are global
// variables to receive data from http protocol
//
// BUILD:
// go build -o lib/hello.so -buildmode=plugin lib/src/hello.go
func GETHelloHandler() (ret string) {
	resp := Response{
		HTTPVars: HTTPVars,
		URLQuery: URLQuery,
		MSG:      "Hello plugin caller!",
	}
	respJSON, err := json.Marshal(resp)
	if err != nil {
		return
	}
	ret = string(respJSON)
	return
}

func GETHelloWithStatusHandler() (ret string, code int) {
	resp := Response{
		HTTPVars: HTTPVars,
		URLQuery: URLQuery,
		MSG:      "Hello plugin caller!",
	}
	respJSON, err := json.Marshal(resp)
	if err != nil {
		return
	}
	ret = string(respJSON)
	code = http.StatusAccepted
	return
}
