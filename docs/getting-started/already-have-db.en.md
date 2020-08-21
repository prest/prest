---
date: 2016-04-23T15:21:22+02:00
title: Pre-existing database
type: homepage
menu:
  getting-started:
    parent: "running"
weight: 4
---

Even though there were four ways to install pREST there's mostly two ways to run it.

1. [With the binary or homebrew or go install](/getting-started/running/#with-the-binary-or-homebrew-or-go-install)
1. [With Docker or Docker Compose](/getting-started/running/#with-docker)


### With the binary or homebrew or go install

If you install pREST by downloading the binary or using Homebrew or using go install, you must pass the necessary variables binary as follows:

```sh
PREST_PG_USER=postgres \
PREST_PG_DATABASE=prest \
PREST_PG_PORT=5432 \
PREST_HTTP_PORT=3010 \
prestd # Binary installed
```

### With docker

Considering you already did the pull in the previous step:

```sh
docker run -e PREST_HTTP_PORT=3000 \
	-e PREST_PG_HOST=127.0.0.1 \
	-e PREST_PG_USER=postgres \
	-e PREST_PG_PASS=pass \
	-e PREST_PG_DATABASE=prest \
	prest/prest:v1
```
if you want to connect to a database running on the host machine you can add `--network host` param.

or if use Docker Compose (there's an [example in the repository](https://github.com/prest/prest/blob/master/docker-compose-prod.yml))

```sh
docker-compose up
```

For more details on how to configure and other environment variables got to [Configurations](/configurations)
