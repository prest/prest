---
title: "Plugins"
date: 2017-08-30T19:07:12-03:00
weight: 3
description: >
  prestd is an extensible software via plugins (OpenCore model)
---

_**prestd**_ is an extensible software via plugins (**OpenCore model**), we use standard operating system library for new functionality.

[Here](https://github.com/prest/prest/discussions/466#discussion-30623) is a discussion of how we arrived at this architecture.

It is possible to create custom **endpoints and middleware** by writing an operating system library (`.so`) and _**prestd**_ can load it when starting the server (api).

> We use the [plugin system of the Go language](https://pkg.go.dev/plugin) to load the lib, unfortunately it doesn't work well with _Microsoft Windows_ yet - if you are working with _**prestd**_ on Windows the plugin endpoint will not exist.

When starting the _**prestd**_ server and there is a plugin in the `./lib` folder they are automatically compiled and loaded when accessing their respective endpoint for the first time.

**Change where the libraries will be:**
`PREST_PLUGINPATH` is the name of the _environment variable_ that has this purpose, by default it comes with the value `./lib`.

or via `toml`:

```
pluginpath = ./lib
```


## Extension-supported modules

* [Endpoint](/prestd/plugins/http-endpoints.md)
* [Middleware](/prestd/plugins/http-endpoints.md)
