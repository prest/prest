---
date: 2016-04-23T15:21:22+02:00
title: Getting Started
type: homepage
menu: main
weight: 10
---

## What is pREST?

pREST is a way to serve a RESTful API from any databases.

We started with support for Postgres (internal need of [Nuveo](https://github.com/nuveo/)), today we have support for adapters, [database list that we supported](/#databases-supported).

## Problem

There is PostgREST written in Haskell, but keeping Haskell software in production is not an easy job. With this need pREST was born.

[Read more](https://github.com/prest/prest/issues/41).

## Databases supported

- PostgreSQL
  - 9.4 or higher
- MySQL ([development](https://github.com/prest/prest/issues/239))

## Supported Operating system

- Linux
  - i386
  - AMD 64
  - ARM 5
  - ARM 6
  - ARM 7
  - ARM 64
  - MIPS
  - MIPS 64
  - MIPS LE
  - MIPS 64 LE
- macOS
  - i386
  - AMD 64
- Windows
  - i386
  - AMD 64
- BSD ([we need your help](https://github.com/prest/prest/issues/279))

## Installation

### binary

Download the latest version [here](https://github.com/prest/prest/releases/latest).

### homebrew


```sh
brew install prest
```

### go get
```sh
go get -u github.com/prest/prest
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
docker pull prest/prest
docker run -e PREST_HTTP_PORT=3000 \
	-e PREST_PG_HOST=127.0.0.1 \
	-e PREST_PG_USER=postgres \
	-e PREST_PG_PASS=pass \
	prest/prest
```

### Versions

- lastest (developer)
- [0.3.0](https://github.com/prest/prest/releases/tag/v0.3.0) (latest stable release)
