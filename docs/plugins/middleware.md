---
title: "Middleware Plugin"
date: 2022-01-04T17:28:24-03:00
weight: 2
---

With prestd's middleware plugin system it is possible to create new middlewares to process before reaching the http handler (endpoint).


## Naming patterns

The prestd configuration file receives two parameters:

- **File name:** name of the `.so` file to be loaded into the library path (`file` in prestd config file), the root directory of middleware is the direct libraries (by default is `./lib`) **+** `/middlewares` folder
- **Function name:** function name which will be loaded (`{func}MiddlewareLoad`, is actually the prefix of the function name)

### Filename

The file name will be used in the middleware to identify which library will be loaded **when the server (prestd) loads**.

> After the **server loads, the library is not loaded again**, it is just executed, i.e. if the file (`.so`) is changed after the server loads, the changes are not applied.

### Function name

When talking about a compiled _library_ we have no way of identifying its functions. Given this characteristic we have defined some name and behavior patterns to develop _libraries_ for _**prestd**_.

**function name:** `{Function Name}MiddlewareLoad`

- `{Function Name}`: The name of the function that will be called
- `MiddlewareLoad`: The suffix of the function name - it is always `negroni.HandlerFunc`

> `fmt.Sprintf("%sMiddlewareLoad", funcName)`

## Example

- **Source code name:** `./lib/src/middlewares/hello.go`
- **Library file name:** `./lib/middlewares/hello.so`
- **Function name:** `HelloMiddlewareLoad`

```go
// nolint
// all plugins must have their package name as `main`
// each plugin is isolated at compile time
package main

import (
	"net/http"

	"github.com/urfave/negroni/v3"
)

// BUILD:
// go build -o ./lib/midllewares/hello.so -buildmode=plugin ./lib/src/middlewares/hello.go
func HelloMiddlewareLoad() negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		rw.Header().Add("X-Hello-Middleware", "Hello Middleware")
		next(rw, rq)
	})
}
```
