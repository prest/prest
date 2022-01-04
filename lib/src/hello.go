// all plugins must have their package name as `main`
// each plugin is isolated at compile time
package main

import (
	"fmt"
)

var (
	// HttpVars route variables for the current request
	HTTPVars map[string]string
	// URLQuery parses RawQuery and returns the corresponding values
	URLQuery map[string][]string
)

// GETHelloHandler plugin
// function is invoked via [go language plugin](https://pkg.go.dev/plugin),
// it is not possible to pass parameters, that's why there are global
// variables to receive data from http protocol
//
// BUILD:
// go build -o lib/hello.so -buildmode=plugin lib/src/hello.go
func GETHelloHandler() (ret string) {
	for k, v := range HTTPVars {
		ret += fmt.Sprintf("http var: %s / %s\n", k, v)
	}
	for k, v := range URLQuery {
		ret += fmt.Sprintf("url query: %s / %s\n", k, v)
	}
	ret += "Hello plugin caller!"
	return
}
