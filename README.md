# pREST
[![Build Status](https://travis-ci.org/nuveo/prest.svg?branch=master)](https://travis-ci.org/nuveo/prest)
[![GoDoc](https://godoc.org/github.com/nuveo/prest?status.png)](https://godoc.org/github.com/nuveo/prest)
[![Go Report Card](https://goreportcard.com/badge/github.com/nuveo/prest)](https://goreportcard.com/report/github.com/nuveo/prest)
[![codecov](https://codecov.io/gh/nuveo/prest/branch/master/graph/badge.svg)](https://codecov.io/gh/nuveo/prest)
[![Gitter](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/nuveo/prest?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

Serve a RESTful API from any PostgreSQL database

## Postgres version

- 9.4 or higher

## Problem

There is the PostgREST written in haskell, keep a haskell software in production is not easy job, with this need that was born the pREST.

## Docker

https://hub.docker.com/r/nuveo/prest/

```
docker run -e PREST_HTTP_PORT=3000 \
	-e PREST_PG_HOST=127.0.0.1 \
	-e PREST_PG_USER=postgres \
	-e PREST_PG_PASS=pass \
	nuveo/prest:0.2
```

### Tags

- 0.2 (stable)
- 0.1 (stable)
- lastest (developer)

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
key = "secret"

[pg]
host = "127.0.0.1"
user = "postgres"
pass = "mypass"
port = 5432
database = "prest"
```

## API's
HEADER:

- JWT middleware is enable by default. To disable JWT need to run pREST in debug mode

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.TJVA95OrM7E2cBab30RMHrHDcEfxjoYZgeFONFh7HgQ
```

## Debug Mode

- Set environment variable `PREST_DEBUG`

```
PREST_DEBUG=true
```

### Filter (WHERE)

```
GET /DATABASE/SCHEMA/TABLE?FIELD=$eq.VALUE
```

Query Operators:

| Name | Description |
|-------|-------------|
| $eq | Matches values that are equal to a specified value.|
| $gt | Matches values that are greater than a specified value.|
| $gte | Matches values that are greater than or equal to a specified value.|
| $lt | Matches values that are less than a specified value.|
| $lte | Matches values that are less than or equal to a specified value.|
| $ne | Matches all values that are not equal to a specified value.|
| $in | Matches any of the values specified in an array.|
| $nin | Matches none of the values specified in an array.|
| $null | Matches if field is null|
| $notnull | Matches if field is not null|


### Filter (WHERE) with JSONb field

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD->>JSONFIELD:jsonb=VALUE (filter)
```

### Select - GET

```
http://127.0.0.1:8000/databases (show all databases)
http://127.0.0.1:8000/databases?_count=* (count all databases)
http://127.0.0.1:8000/databases?_renderer=xml (JSON by default)
http://127.0.0.1:8000/schemas (show all schemas)
http://127.0.0.1:8000/schemas?_count=* (count all schemas)
http://127.0.0.1:8000/schemas?_renderer=xml (JSON by default)
http://127.0.0.1:8000/tables (show all tables)
http://127.0.0.1:8000/tables?_renderer=xml (JSON by default)
http://127.0.0.1:8000/DATABASE/SCHEMA (show all tables, find by schema)
http://127.0.0.1:8000/DATABASE/SCHEMA?_renderer=xml (JSON by default)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE (show all rows, find by database and table)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_select=column (select statement by columns)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_select=column[array id] (select statement by array colum)

http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_select=* (select all from TABLE)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_count=* (use count function)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_count=column (use count function)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_page=2&_page_size=10 (pagination, page_size 10 by default)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD=VALUE (filter)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_renderer=xml (JSON by default)


Select operations over a VIEW
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_select=column (select statement by columns in VIEW)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_select=* (select all from VIEW)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_count=* (use count function)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_count=column (use count function)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_page=2&_page_size=10 (pagination, page_size 10 by default)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?FIELD=VALUE (filter)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_renderer=xml (JSON by default)

```

### Insert - POST

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE
```

JSON DATA:
```
{
    "FIELD1": "string value",
    "FIELD2": 1234567890
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
    "FIELD1": "string value",
    "FIELD2": 1234567890,
    "ARRAYFIELD": ["value 1","value 2"]
}
```

### Delete - DELETE

Using query string to make filter (WHERE), example:

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD1=xyz
```

## JOIN

Using query string to JOIN tables, example:

```
/DATABASE/SCHEMA/TABLE?_join=inner:users:friends.userid:$eq:users.id
```

Parameters:

1. Join type
1. Table
1. Table field 1
1. Operator (=, <, >, <=, >=)
1. Table field 2

## Query Operators

| Name | Description |
|-------|-------------|
| $eq | Matches values that are equal to a specified value.|
| $gt | Matches values that are greater than a specified value.|
| $gte | Matches values that are greater than or equal to a specified value.|
| $lt | Matches values that are less than a specified value.|
| $lte | Matches values that are less than or equal to a specified value.|
| $ne | Matches all values that are not equal to a specified value.|
| $in | Matches any of the values specified in an array.|
| $nin | Matches none of the values specified in an array.|

## ORDER BY

Using *ORDER BY* in queries you must pass in *GET* request the attribute `_order` with fieldname(s) as value. For *DESC* order, use the prefix `-`. For *multiple* orders, the fields are separated by comma.

Examples:

### ASC
    GET /DATABASE/SCHEMA/TABLE/?_order=fieldname

### DESC
    GET /DATABASE/SCHEMA/TABLE/?_order=-fieldname

### Multiple Orders
    GET /DATABASE/SCHEMA/TABLE/?_order=fieldname01,-fieldname02,fieldname03


## GROUP BY

We support this Group Functions:

| name | Use in request |
| ------- | ------------- |
| SUM | sum:field |
| AVG | avg:field |
| MAX | max:field |
| MIN | min:field |
| MEDIAN | median:field |
| STDDEV | stddev:field |
| VARIANCE | variance:field |

### Examples:
	GET /DATABASE/SCHEMA/TABLE/?_select=fieldname00,fieldname01&_groupby=fieldname01

#### Using Group Functions
	GET /DATABASE/SCHEMA/TABLE/?_select=fieldname00,sum:fieldname01&_groupby=fieldname01



## Executing SQL scripts

If need perform an advanced SQL, you can write some scripts SQL and access them by REST. These scripts are templates where you can pass by URL, values to them.

_awesome_folder/example_of_powerful.read.sql_:
```sql
SELECT * FROM table WHERE name = "{{.field1}}" OR name = "{{.field2}}";
```

Get result:

```
GET /_QUERIES/awesome_folder/example_of_powerful?field1=foo&field2=bar
```

To activate it, you need configure a location to scripts in your prest.toml like:

```
[queries]
location = /path/to/queries/
```

### Scripts templates rules

In your scripts, the fields to replace have to look like: _field1 or field2 are examples_

```sql
SELECT * FROM table WHERE name = "{{.field1}}" OR name = "{{.field2}}";
```

Script file must have a suffix based on http verb:

|HTTP Verb|Suffix|
|---|---|
|GET|.read.sql|
|POST|.write.sql|
|PUT, PATCH|.update.sql|
|DELETE|.delete.sql|

In `queries.location`, you need given a folder to your scripts:

```
queries/
└── foo
    └── some_get.read.sql
    └── some_create.write.sql
    └── some_update.update.sql
    └── some_delete.delete.sql
└── bar
    └── some_get.read.sql
    └── some_create.write.sql
    └── some_update.update.sql
    └── some_delete.delete.sql

URL's to foo folder:

GET    /_QUERIES/foo/some_get?field1=bar
POST   /_QUERIES/foo/some_create?field1=bar
PUT    /_QUERIES/foo/some_update?field1=bar
PATCH  /_QUERIES/foo/some_update?field1=bar
DELETE /_QUERIES/foo/some_delete?field1=bar


URL's to bar folder:

GET    /_QUERIES/bar/some_get?field1=foo
POST   /_QUERIES/bar/some_create?field1=foo
PUT    /_QUERIES/bar/some_update?field1=foo
PATCH  /_QUERIES/bar/some_update?field1=foo
DELETE /_QUERIES/bar/some_delete?field1=foo
```
### Template functions

- *isSet* return true if param is set

```sql
SELECT * FROM table 
{{if isSet "field1"}}
WHERE name = "{{.field1}}"
{{end}} 
;
```

- *defaultOrValue* return param value or default value

```sql
SELECT * FROM table WHERE name = '{{defaultOrValue "field1" "gopher"}}';
```

## Permissions

### Restrict mode
In the prest.toml you can configure read/write/delete permissions of each table.

```
[access]
restrict = true  # can access only the tables listed below
```

`restrict = false`: (default) the prest will serve in publish mode. You can write/read/delete everydata without configure permissions.

`restruct = true`: you need configure the permissions of all tables.

### Table permissions

Example:

```
[[access.tables]]
name = "test"
permissions = ["read", "write", "delete"]
fields = ["id", "name"]
```

|attribute|description|
|---|---|
|table|Table name|
|permissions|Table permissions. Options: `read`, `write` and `delete`|
|fields|Fields permitted for select|


Configuration example: [prest.toml](https://github.com/nuveo/prest/blob/master/testdata/prest.toml)


## CORS Support

In the prest.toml you can configurate the CORS allowed origin:

Example:

```
[cors]
alloworigin = ["http://postgres.rest", "http://foo.com"]
```
