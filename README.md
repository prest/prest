# <img align="right" src="https://docs.prestd.com/logo.png" alt="RESTful API" title="RESTful API"> pREST

[![Build Status](https://travis-ci.com/prest/prest.svg?branch=main)](https://travis-ci.com/prest/prest)
[![GoDoc](https://godoc.org/github.com/prest/prest?status.png)](https://godoc.org/github.com/prest/prest)
[![Go Report Card](https://goreportcard.com/badge/github.com/prest/prest)](https://goreportcard.com/report/github.com/prest/prest)
[![codecov](https://codecov.io/gh/prest/prest/branch/main/graph/badge.svg?token=eVD9urwIEv)](https://codecov.io/gh/prest/prest)
[![SourceLevel](https://app.sourcelevel.io/github/prest/-/prest.svg)](https://app.sourcelevel.io/github/prest/-/prest)
[![Homebrew](https://img.shields.io/badge/dynamic/json.svg?url=https://formulae.brew.sh/api/formula/prestd.json&query=$.versions.stable&label=homebrew)](https://formulae.brew.sh/formula/prestd)
[![Slack](https://img.shields.io/badge/slack-prestd-blueviolet.svg?logo=slack)](http://slack.prestd.com/)

_p_**REST** (**P**_ostgreSQL_ **REST**), simplify and accelerate development, instant, realtime, high-performance on any **Postgres** application, **existing or new**

> PostgreSQL version 9.4 or higher

<a href="https://www.producthunt.com/posts/prest?utm_source=badge-featured&utm_medium=badge&utm_souce=badge-prest" target="_blank"><img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=303506&theme=light" alt="pREST - instant, realtime, high-performance on PostgreSQL | Product Hunt" style="width: 250px; height: 54px;" width="250" height="54" /></a>

## Problem

There is PostgREST written in Haskell, but keeping Haskell software in production is not an easy job. With this need pREST was born. [Read more](https://github.com/prest/prest/issues/41).

## Test using Docker

> _To simplify the process of bringing up the test environment we will use **docker-compose**, assuming you do not have the repository cloned locally, we are assuming you are reading this page for the first time_

```sh
# Download docker compose file
wget https://raw.githubusercontent.com/prest/prest/main/docker-compose-prod.yml -O docker-compose.yml

# Up (run) PostgreSQL and prestd
docker-compose up
# Run data migration to create user structure for access (JWT)
docker-compose exec prest prestd migrate up auth

# Create user and password for API access (via JWT)
## user: prest
## pass: prest
docker-compose exec postgres psql -d prest -U prest -c "INSERT INTO prest_users (name, username, password) VALUES ('pREST Full Name', 'prest', MD5('prest'))"
# Check if the user was created successfully (by doing a select on the table)
docker-compose exec postgres psql -d prest -U prest -c "select * from prest_users"

# Generate JWT Token with user and password created
curl -i -X POST http://127.0.0.1:3000/auth -H "Content-Type: application/json" -d '{"username": "prest", "password": "prest"}'
# Access endpoint using JWT Token
curl -i -X GET http://127.0.0.1:3000/prest/public/prest_users -H "Accept: application/json" -H "Authorization: Bearer {TOKEN}"
```

## Development usage

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

### Running local unit tests (integration/e2e)

pREST's unit tests depend on a working postgres database for sql query execution, to simplify the preparation of the local environment we use docker (and docker-compose) to upload the environment with postgres.

**all tests:**

```sh
docker-compose -f testdata/docker-compose.yml up --abort-on-container-exit
```

**package-specific testing:**
_in the example below the `config` package will be tested_

```sh
docker-compose -f testdata/docker-compose.yml run prest-test sh ./testdata/runtest.sh ./config
```

**specific function test:**
_in the example below will run the test `TestGetDefaultPrestConf` from the `config` package_

```sh
docker-compose -f testdata/docker-compose.yml run prest-test sh ./testdata/runtest.sh ./config -run TestGetDefaultPrestConf
```

## 1-Click Deploy

### Heroku

Deploy to Heroku and instantly get a realtime RESTFul API backed by Heroku Postgres:

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/prest/prest-heroku)

## Documentation

<https://docs.prestd.com/> ([content source](https://github.com/prest/prest/tree/main/docs) and [template source](https://github.com/prest/doc-template))
