---
date: 2016-04-23T15:21:22+02:00
title: Getting Started
type: homepage
menu: main
weight: 1
---

## Installation

### Binary

For any OS you can download the latest version [here](https://github.com/prest/prest/releases/latest).

### go get

```sh
go get -u github.com/prest/prest
```

### MacOS

If none of the above suits you, there's still an option of installing using [Homebrew](https://brew.sh/)

```sh
brew install prest
```

## With docker

We only will need to download the pREST image from Docker Hub with:

```sh
docker pull prest/prest
```

## Running

### With the binary or homebrew or go get

You can pass the necessary variables to the binary as follows:

```sh
PREST_PG_USER=postgres \
PREST_PG_DATABASE=prest \
PREST_PG_PORT=5432 \
PREST_HTTP_PORT=3010 \
prest # Binary installed
```

### With docker

Considering you already did the pull in the previous step:

```sh
docker run -e PREST_HTTP_PORT=3000 \
	-e PREST_PG_HOST=127.0.0.1 \
	-e PREST_PG_USER=postgres \
	-e PREST_PG_PASS=pass \
	prest/prest
```
or if use Docker Compose (there's an [example in the repository](https://github.com/prest/prest/blob/master/docker-compose.yml))

```sh
docker-compose up
```

For more details on how to configure and other environment variables got to [Configurations](/configurations)
