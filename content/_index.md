---
date: 2016-04-23T15:21:22+02:00
title: Getting Started
type: homepage
menu: main
weight: 10
---

## What is pREST?

pREST is a way to serve a RESTful API from any PostgreSQL 9.4+ database.

## Installation

```sh
go get github.com/prest/prest
```

## Running

Initally can use some environment variables by example:

- PREST\_HTTP_PORT (default 3000)
- PREST\_PG_HOST (default 127.0.0.1)
- PREST\_PG_USER
- PREST\_PG_PASS
- PREST\_PG_DATABASE
- PREST\_PG_PORT (default 5432)
- PREST\_JWT_KEY

```sh
PREST_PG_USER=postgres \ 
PREST_PG_DATABASE=prest \
PREST_PG_PORT=5432 \
PREST_HTTP_PORT=3010 \
prest # Binary installed
```

In case needs use it via Docker: https://hub.docker.com/r/prest/prest/

```sh
docker run -e PREST_HTTP_PORT=3000 \
	-e PREST_PG_HOST=127.0.0.1 \
	-e PREST_PG_USER=postgres \
	-e PREST_PG_PASS=pass \
	prest/prest:0.2
```

### Tags

- [0.3](https://github.com/prest/prest/releases/tag/v0.3.0) (latest stable release)
- 0.2
- 0.1
- lastest (developer)
