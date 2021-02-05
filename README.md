# <img align="right" src="https://docs.prestd.com/logo.png" alt="RESTful API" title="RESTful API"> pREST
[![Build Status](https://travis-ci.org/prest/prest.svg?branch=master)](https://travis-ci.org/prest/prest)
[![GoDoc](https://godoc.org/github.com/prest/prest?status.png)](https://godoc.org/github.com/prest/prest)
[![Go Report Card](https://goreportcard.com/badge/github.com/prest/prest)](https://goreportcard.com/report/github.com/prest/prest)
[![Coverage Status](https://coveralls.io/repos/github/prest/prest/badge.svg?branch=master)](https://coveralls.io/github/prest/prest?branch=master)
[![SourceLevel](https://app.sourcelevel.io/github/prest/-/prest.svg)](https://app.sourcelevel.io/github/prest/-/prest)
[![Homebrew](https://img.shields.io/badge/dynamic/json.svg?url=https://formulae.brew.sh/api/formula/prestd.json&query=$.versions.stable&label=homebrew)](https://formulae.brew.sh/formula/prestd)

_p_**REST** (**P**_ostgreSQL_ **REST**), simplify and accelerate development, instant, realtime, high-performance on any **Postgres** application, **existing or new**

## Postgres version

- 9.4 or higher

## Problem

There is PostgREST written in Haskell, but keeping Haskell software in production is not an easy job. With this need pREST was born. [Read more](https://github.com/prest/prest/issues/41).

## Development usage

For dependencies installation

```sh
go mod download
```

Building a local version

```sh
go build -ldflags "-s -w" -o prestd cmd/prestd/main.go
```

Executing the prestd

```sh
PREST_PG_USER=postgres PREST_PG_PASS=postgres PREST_PG_DATABASE=prest PREST_PG_PORT=5432 PREST_HTTP_PORT=3010 ./prestd
```

or use 'prest.toml' as a preset configuration, insert a user to see the changes

```sh
./prestd migrate up auth
```

```sh
INSERT INTO prest_users (name, username, password) VALUES ('prest', 'prest', MD5('prest'));
```

Now you can authenticate

```sh
curl -i -X POST http://127.0.0.1:3010/auth -H "Content-Type: application/json" -d '{"username": "prest", "password": "prest"}'
```

```sh
curl -i X GET http://127.0.0.1:3010/prest/public/prest_users -H "Accept: application/json" -H "Authorization: Bearer {token}"
```

## 1-Click Deploy

### Heroku
Deploy to Heroku and instantly get a realtime RESTFul API backed by Heroku Postgres:

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/prest/prest-heroku)

## Documentation

https://docs.prestd.com/ ([content source](https://github.com/prest/prest/tree/master/docs) and [template source](https://github.com/prest/doc-template))
