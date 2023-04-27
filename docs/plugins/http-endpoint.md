---
title: "Endpoint Plugin"
date: 2022-01-04T17:28:24-03:00
weight: 1
---

With prestd's http plugin system it is possible to create new endpoints in the "private" URI,

The plugin endpoint has the following default: `/_PLUGIN/{file}/{func}`

## Naming patterns

The plugin endpoint (`/_PLUGIN/{file}/{func}`) receives two parameters:

- File name: The name of the file without the extension (`{file}`)
- Function name: The name of the function (`{func}`)

### File name

The file name will be used on the endpoint to identify which library will be loaded when it receives the first access.

> After the **first access the library will not be loaded again**, it will only be executed, i.e., if the file (`.so`) is changed after the first execution it will have no effect because it has already been loaded.

### Function name

When talking about a compiled _library_ we have no way of identifying its functions. Given this characteristic we have defined some name and behavior patterns to develop _libraries_ for _**prestd**_.

**function name:** `{HTTP Method}{Function Name}Handler`

- `{HTTP Method}`: The HTTP method that the function will be called for (in upper case letters)
- `{Function Name}`: The name of the function that will be called
- `Handler`: The suffix of the function name - it is always `Handler`

> `fmt.Sprintf("%s%sHandler", r.Method, funcName)`

## Process of building

In the first version of the _**prestd**_ plugin system we are working with **Go code**.

> This doesn't mean that _**prestd**_ doesn't read plugins (library `.so`) written in other technologies (e.g. c, cpp, java and ...).
> The automatic constructor is designed to work with Go code, in the future we will write for other technologies.

## Example

- **Source code name:** `./lib/src/hello.go`
- **Library file name:** `./lib/hello.so`
- **Function name:** `GETHelloHandler`
- **Endpoint:** `/_PLUGIN/hello/Hello`
- **Verb HTTP:** `GET`

```go
// all plugins must have their package name as `main`
// each plugin is isolated at compile time
package main

import (
 "encoding/json"
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

// GETHelloHandler plugin
// same function as GETHelloHandler, but this time we can return status code.
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
```

**Request:**

```http
GET /_PLUGIN/hello/Hello?abc=123 HTTP/1.1
```

**Response:**

```json
{
  "http_vars": {
    "file": "hello",
    "func": "Hello"
  },
  "url_query": {
    "abc": [
      "123"
    ]
  },
  "msg": "Hello plugin caller!"
}
```
