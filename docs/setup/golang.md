---
date: 2016-04-23T15:21:22+02:00
title: Golang
weight: 5
description: >
  prestd can be deployed locally using Golang environment
---

## Prerequisites

- Go (1.7+)

---

## Quick Start

The `go install` command builds and installs the packages named by the paths on the command line. Executables (main packages) are installed to the directory named by the GOBIN environment variable, which defaults to `$GOPATH/bin` or `$HOME/go/bin` if the **GOPATH** environment variable is not set. Executables in `$GOROOT` are installed in `$GOROOT/bin` or `$GOTOOLDIR` instead of `$GOBIN`. Non-executable packages are built and cached but not installed.

Since **Go 1.16**, if the arguments have version suffixes (like `@latest` or `@v1.0.0`), go install builds packages in module-aware mode, ignoring the go.mod file in the current directory or any parent directory if there is one. This is useful for installing executables without affecting the dependencies of the main module.

```sh
go install github.com/prest/prest/cmd/prestd@latest
```
