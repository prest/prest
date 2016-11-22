# pREST
Serve a RESTful API from any PostgreSQL database

## Problem
There is the PostgREST written in haskell, keep a haskell software in production is not easy job, with this need that was born the pREST.

## API`s

```
	GET http://127.0.0.1:8000/DATABASE/ (show all tables)
	GET http://127.0.0.1:8000/DATABASE/TABLE (select all)
	GET http://127.0.0.1:8000/DATABASE/TABLE?_page=2&_page_size=10 (pagination, page_size 10 by default)
	GET http://127.0.0.1:8000/DATABASE/TABLE?FIELD=VALUE (filter)
	GET http://127.0.0.1:8000/DATABASE/TABLE?_renderer=xml (JSON by default)
```
