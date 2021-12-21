---
title: "Run pREST using Docker"
date: 2017-08-30T19:06:24-03:00
type: homepage
menu: deployment
weight: 3
---

This guide assumes that you already have Postgres running and helps you set up the pREST using Docker and connect it to your Postgres database.

In case you’d like to run pREST with a fresh Postgres database, follow this guide to deploy the pREST along with a Postgres instance using _Docker Compose_.

## Deploying prestd using Docker

**Prerequisites:**

- [Docker](https://docs.docker.com/get-docker/)

### Docker Run

We will use docker to run pREST and connect to an existing database.
To simplify the example we leave the authentication module off

```shell
docker run -d -p 3000:3000 \
    -e PREST_PG_URL=postgres://username:password@hostname:port/dbname \
    -e PREST_DEBUG=true \
    prest/prest:v1
```

Edit the `PREST_PG_URL` env var value, so that you can connect to your Postgres instance.

Examples of `PREST_PG_URL`:

- postgres://admin:password@localhost:5432/my-db
- postgres://admin:@localhost:5432/my-db _(if there is no password)_

> If your password contains special characters (e.g. #, %, $, @, etc.), you need to URL encode them in the `PREST_PG_URL` env var (e.g. %40 for @).
> You can check the logs to see if the database credentials are proper and if pREST is able to connect to the database.
> pREST needs access permissions to your Postgres database as described in [permissions page](/permissions/).

### Network config

If your Postgres instance is running on `localhost`, the following changes will be needed to the `docker run` command to allow the Docker container to access the host’s network.

Add the `--net=host` flag to access the host’s Postgres service.

This is what your command should look like:

```shell
docker run -d --net=host -p 3000:3000 \
    -e PREST_PG_URL=...
```

> if you are using another operating system we recommend reading the [docker network documentation](https://docs.docker.com/network/host/), on **macOS** and **Windows** it is different.

### this done, call the API

Example using **curl:**

```shell
curl -i -X GET http://127.0.0.1:3000/databases -H "Content-Type: application/json"
```

## Docker Compose

> Compose is a tool for defining and running multi-container Docker applications. With Compose, you use a YAML file to configure your application’s services. Then, with a single command, you create and start all the services from your configuration. To learn more about all the features of Compose, see the [list of features](https://docs.docker.com/compose/#features).

{{< emgithub https://github.com/prest/prest/blob/main/docker-compose-prod.yml >}}
