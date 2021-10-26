---
title: "Extending (framework)"
date: 2017-08-30T19:07:05-03:00
weight: 15
menu: main
chapter: true
---

pREST was developed with the possibility of using it as a web framework, being able to use it based on its API, you create new endpoints and place middleware, adapting the pREST to your need.

### Sample Hello World

In order to create custom modules for pREST you need extends the router and register the custom new routes.

```go
package main

import (
	"net/http"

	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/cmd"
	"github.com/prest/prest/config"
	"github.com/prest/prest/config/router"
	"github.com/prest/prest/middlewares"
)

func main() {
	config.Load()

	// Load Postgres Adapter
	postgres.Load()

	// Get pREST app
	middlewares.GetApp()

	// Get pPREST router
	r := router.Get()

	// Register custom routes
	r.HandleFunc("/ping", Pong).Methods("GET")

	// Call pREST cmd
	cmd.Execute()
}

func Pong(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Pong!"))
}
```

### API routes

All you need to know about all the API routes, please view the following links for more details.

| Route | Description | Method | Details |
| ----------- | ----------- | ----------- | ----------- |
| `/databases` | databases list | GET | [link](#databases) |
| `/schemas` | schemas list | GET | [link](#schemas) |
| `/tables` | tables list | GET | [link](#tables) |
| `/_QUERIES/custom/{script}` | executes a custom query | *depends* | [link](#custom-queries) |
| `/{database}/{schema}` | schema of a database | GET | [link](#select-schema) |
| `/{database}/{schema}/{table}` | select fields from a table | GET | [link](#select-from-table) |
| `/{database}/{schema}/{table}` | insert into a table | POST | [link](#insert-into-table) |
| `/batch/{database}/{schema}/{table}` | batch insert into a table | POST | [link](#batch-insert-on-table) |
| `/{database}/{schema}/{table}` | delete from table | DELETE | [link](#delete-from-table) |
| `/{database}/{schema}/{table}` | update on table | PUT, PATCH | [link](#update-on-table) |

## databases

Should return all databases visible in json list format.

1. Path: `/databases`
2. Method: `GET`
3. Response: json
4. HTTP responses: 200, 401

## schemas

Should return all schemas available in the databases in json list format.

1. Path: `/schemas`
2. Method: `GET`
3. Response: json
4. HTTP responses: 200, 401


## tables

Should return all tables available in the databases in json list format.

1. Path: `/tables`
2. Method: `GET`
3. Response: json
4. HTTP responses: 200, 401

## custom queries

Premium feature, should always return a json according to the custom query given.

1. Path: `/tables`
2. Method: `depends on the custom query`
3. Response: json
4. HTTP responses: 200, 201, 401

## select schema

Should return the database schema

1. Path: `/{database}/{schema}`
2. Method: `GET`
3. Response: json
4. HTTP responses: 200, 401

## CRUD operations

CRUD operations over tables can carry url query params to allow higher complexity queries, they are listed bellow. Possible query URL params:

- `_columns`: column names to get from table
- `_distinct`: if the returns should have distinct values (needs to be set to "true")
- `_groupby`: if the results should be grouped by a column, value should be the name of a column
- `_orderby`: if the results should be ordered by a column, value should be the name of a column
- `_count`: if the results should count a value of a column, value should be the name of a column
- `_join`: if the result should make a join operation with another table, this value has 5 arguments divided by ':'. Example: `https://example.com/?_join=<join_type(inner|left|right)>:<schema>.<table_B>:<table_B>.<column>:<operator(=)>:<table_A>.<column>`
- `_page`: if the result should be paginated, this is the number of the page to be returned
- `_page_size`: number of objects to be returned on a paginated response


> where by clause:
> columns can be filtered by using the `column=<value>` expression on url params


### select from table

Should get information from a table using the CRUD query params as filters.

1. Path: `/{database}/{schema}/{table}`
2. Method: `GET`
3. Response: json
4. HTTP responses: 200, 401

### insert into table

Should try to insert the json object given on body to the given table on path.

1. Path: `/{database}/{schema}/{table}`
2. Method: `POST`
3. Response: json
4. HTTP responses: 200, 400, 401

### delete from table

Should try to delete a 

1. Path: `/{database}/{schema}/{table}`
2. Method: `POST`
3. Response: json
4. HTTP responses: 200, 400, 401

### update on table

Should try to update a table with the given url query params.

1. Path: `/{database}/{schema}/{table}`
2. Method: `PUT, PATCH`
3. Response: json
4. HTTP responses: 200, 400, 401

### batch insert on table

Should try to do the insert batch on a table, using the request body as the insert data.

1. Path: `/batch/{database}/{schema}/{table}`
2. Method: `POST`
3. Response: json
4. HTTP responses: 200, 400, 401
