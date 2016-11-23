# pREST
Serve a RESTful API from any PostgreSQL database

## Problem
There is the PostgREST written in haskell, keep a haskell software in production is not easy job, with this need that was born the pREST.

## API`s

```
GET http://127.0.0.1:8000/databases (show all databases)
GET http://127.0.0.1:8000/schemas (show all schemas)
GET http://127.0.0.1:8000/tables (show all tables)
GET http://127.0.0.1:8000/DATABASE/SCHEMA (show all schemas, find by database)
GET http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE (show all tables, find by database and table)
GET http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_page=2&_page_size=10 (pagination, page_size 10 by default)
GET http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD=VALUE (filter)
GET http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_renderer=xml (JSON by default)
```
