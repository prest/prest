# pREST
[![Build Status](https://travis-ci.org/nuveo/prest.svg?branch=master)](https://travis-ci.org/nuveo/prest)
[![GoDoc](https://godoc.org/github.com/nuveo/prest?status.png)](https://godoc.org/github.com/nuveo/prest)
[![Go Report Card](https://goreportcard.com/badge/github.com/nuveo/prest)](https://goreportcard.com/report/github.com/nuveo/prest)
[![codecov](https://codecov.io/gh/nuveo/prest/branch/master/graph/badge.svg)](https://codecov.io/gh/nuveo/prest)

Serve a RESTful API from any PostgreSQL database

## Problem
There is the PostgREST written in haskell, keep a haskell software in production is not easy job, with this need that was born the pREST.

## Install

    go get github.com/nuveo/prest

## Run

Params:

- PREST\_HTTP_PORT (default 3000)
- PREST\_PG_HOST (default 127.0.0.1)
- PREST\_PG_USER
- PREST\_PG_PASS
- PREST\_PG_DATABASE
- PREST\_PG_PORT (default 5432)
- PREST\_JWT_KEY

```
PREST_PG_USER=postgres PREST_PG_DATABASE=prest PREST_PG_PORT=5432 PREST_HTTP_PORT=3010 prest # Binary installed
```

## Migrations

`--url` and `--path` flags are optional if pREST configurations already set

```bash
# env var for migrations directory
PREST_MIGRATIONS

# create new migration file in path
prest migrate --url driver://url --path ./migrations create migration_file_xyz

# apply all available migrations
prest migrate --url driver://url --path ./migrations up

# roll back all migrations
prest migrate --url driver://url --path ./migrations down

# roll back the most recently applied migration, then run it again.
prest migrate --url driver://url --path ./migrations redo

# run down and then up command
prest migrate --url driver://url --path ./migrations reset

# show the current migration version
prest migrate --url driver://url --path ./migrations version

# apply the next n migrations
prest migrate --url driver://url --path ./migrations next +1
prest migrate --url driver://url --path ./migrations next +2
prest migrate --url driver://url --path ./migrations next +n

# roll back the previous n migrations
prest migrate --url driver://url --path ./migrations next -1
prest migrate --url driver://url --path ./migrations next -2
prest migrate --url driver://url --path ./migrations next -n

# go to specific migration
prest migrate --url driver://url --path ./migrations goto 1
prest migrate --url driver://url --path ./migrations goto 10
prest migrate --url driver://url --path ./migrations goto v
```

## TOML
Optionally the pREST can be configured by TOML file

- Set `PREST_CONF` environment variable with file path

```toml
migrations = "./migrations"

[http]
port = 6000

[jwt]
key = "mysecretkey"

[pg]
host = "127.0.0.1"
user = "postgres"
pass = "mypass"
port = 5432
database = "prest"
```

## API`s
HEADER:

- To start JWT middleware the `PREST_JWT_KEY` environment variable must be set

```
Authorization: JWT eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.TJVA95OrM7E2cBab30RMHrHDcEfxjoYZgeFONFh7HgQ
```

### Filter (WHERE) with JSONb field

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD->>JSONFIELD:jsonb=VALUE (filter)
```

### Select - GET

```
http://127.0.0.1:8000/databases (show all databases)
http://127.0.0.1:8000/schemas (show all schemas)
http://127.0.0.1:8000/tables (show all tables)
http://127.0.0.1:8000/DATABASE/SCHEMA (show all tables, find by schema)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE (show all rows, find by database and table)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_page=2&_page_size=10 (pagination, page_size 10 by default)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD=VALUE (filter)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_renderer=xml (JSON by default)
```

### Insert - POST

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE
```

JSON DATA:
```
{
    "data": {
        "FIELD1": "string value",
        "FIELD2": 1234567890
    }
}
```

### Update - PATCH/PUT

Using query string to make filter (WHERE), example:

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD1=xyz
```

JSON DATA:
```
{
    "data": {
        "FIELD1": "string value",
        "FIELD2": 1234567890
    }
}
```

### Delete - DELETE

Using query string to make filter (WHERE), example:

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD1=xyz
```
