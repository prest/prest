---
title: "Development Guides"
date: 2021-09-02T13:39:24-03:00
weight: 2
---

_**prestd**_ is written in the [go language](https://golang.org) and we use the best practices recommended by the language itself to simplify its contribution.
If you are not familiar with the language, read the [Effective Go](https://golang.org/doc/effective_go).

## Development usage

As mentioned before prest is written in **go**, as it is in the document topic of using prest in development mode it is important to know the go language path structure, if you don't know it read the page [How to Write Go Code (with GOPATH)](https://golang.org/doc/gopath_code).

> Assuming you do not have the [repository cloned](https://github.com/prest/prest "git clone git@github.com:prest/prest.git") locally, we are assuming you are reading this page for the first time_

Download all of pREST's dependencies

```sh
git clone git@github.com:prest/prest.git && cd prest
go mod download
```

We recommend using `go run` for development environment, remember that it is necessary environment variables for _p_**REST** to connect to PostgreSQL - we will explain in the next steps how to do it

```sh
go run cmd/prestd/main.go
```

Building a **local version** (we will not use flags for production environment)

```sh
go build -o prestd cmd/prestd/main.go
```

Executing the `prestd` after generating binary or using `go run`

```sh
PREST_PG_USER=postgres PREST_PG_PASS=postgres PREST_PG_DATABASE=prest PREST_PG_PORT=5432 PREST_HTTP_PORT=3010 ./prestd
```

> to use `go run` replace `./prestd` with `go run`

or use `'prest.toml'` file as a preset configuration, insert a user to see the changes

## Execute unit tests locally (integration/e2e)

pREST's unit tests depend on a working Postgres database for SQL query execution, to simplify the preparation of the local environment we use docker (and docker-compose) to upload the environment with Postgres.

**all tests:**

```sh
docker-compose -f testdata/docker-compose.yml up --abort-on-container-exit
```

**package-specific testing:**
_in the example below the `config` package will be tested_

```sh
docker-compose -f testdata/docker-compose.yml run --rm prest-test sh ./testdata/runtest.sh ./config
```

**specific function test:**
_in the example below will run the test `TestGetDefaultPrestConf` from the `config` package, don't forget to call the `TestMain` function before your function_

```sh
docker-compose -f testdata/docker-compose.yml run prest-test sh ./testdata/runtest.sh ./config -run TestMain,TestGetDefaultPrestConf
```
