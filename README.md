# <img align="right" src="https://docs.prestd.com/logo.png" alt="RESTful API" title="RESTful API"> prestd

[![Build Status](https://travis-ci.com/prest/prest.svg?branch=main)](https://travis-ci.com/prest/prest)
[![GoDoc](https://godoc.org/github.com/prest/prest?status.png)](https://godoc.org/github.com/prest/prest)
[![Go Report Card](https://goreportcard.com/badge/github.com/prest/prest)](https://goreportcard.com/report/github.com/prest/prest)
[![codecov](https://codecov.io/gh/prest/prest/branch/main/graph/badge.svg?token=eVD9urwIEv)](https://codecov.io/gh/prest/prest)
[![Homebrew](https://img.shields.io/badge/dynamic/json.svg?url=https://formulae.brew.sh/api/formula/prestd.json&query=$.versions.stable&label=homebrew)](https://formulae.brew.sh/formula/prestd)
[![Slack](https://img.shields.io/badge/slack-prestd-blueviolet.svg?logo=slack)](http://slack.prestd.com/)

_p_**REST** (**P**_ostgreSQL_ **REST**), simplify and accelerate development, instant, realtime, high-performance on any **Postgres** application, **existing or new**

> PostgreSQL version 9.5 or higher

Contributor License Agreement - [![CLA assistant](https://cla-assistant.io/readme/badge/prest/prest)](https://cla-assistant.io/prest/prest)

<a href="https://www.producthunt.com/posts/prest?utm_source=badge-featured&utm_medium=badge&utm_souce=badge-prest" target="_blank"><img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=303506&theme=light" alt="pREST - instant, realtime, high-performance on PostgreSQL | Product Hunt" style="width: 250px; height: 54px;" width="250" height="54" /></a>

## Problem

There is PostgREST written in Haskell, but keeping Haskell software in production is not an easy job. With this need prestd was born. [Read more](https://github.com/prest/prest/issues/41).

## Test using Docker

> _To simplify the process of bringing up the test environment we will use **docker-compose**_

```sh
# Download docker compose file
wget https://raw.githubusercontent.com/prest/prest/main/docker-compose-prod.yml -O docker-compose.yml

# Up (run) PostgreSQL and prestd
docker-compose up
# Run data migration to create user structure for access (JWT)
docker-compose exec prest prestd migrate up auth

# Create user and password for API access (via JWT)
## user: prest
## pass: prest
docker-compose exec postgres psql -d prest -U prest -c "INSERT INTO prest_users (name, username, password) VALUES ('pREST Full Name', 'prest', MD5('prest'))"
# Check if the user was created successfully (by doing a select on the table)
docker-compose exec postgres psql -d prest -U prest -c "select * from prest_users"

# Generate JWT Token with user and password created
curl -i -X POST http://127.0.0.1:3000/auth -H "Content-Type: application/json" -d '{"username": "prest", "password": "prest"}'
# Access endpoint using JWT Token
curl -i -X GET http://127.0.0.1:3000/prest/public/prest_users -H "Accept: application/json" -H "Authorization: Bearer {TOKEN}"
```

## Samples to getting started with API calls

### Description

First api calls and test automation sample.

### Usage

Import on Postman and execute the following steps:

* Bearer Authentication
* List Databases

This is the manual process to see how things is going.

So, we have the automated way:

```
npm i --location=global newman
```

After the installation run the following command:

```
newman run samples/prest_first_look.postman_collection.json
```

That's it, you have a way to validate the project running locally, and to test
on the environments you need to edit and go forward with your own version of this
sample.

Want to contribute to the project and don't know where to start? See our contribution guide [here](https://docs.prestd.com/contribution/).

## 1-Click Deploy

### Heroku

Deploy to Heroku and instantly get a realtime RESTFul API backed by Heroku Postgres:

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/prest/prest-heroku)

## Documentation

<https://docs.prestd.com/> ([content source](https://github.com/prest/prest/tree/main/docs) and [template source](https://github.com/prest/doc-template))

## run locally

You can use the db from our `docker-compose.yml` file and disable the prestd image pull, then just use the following commands:

```
$ prest git:(main) cd cmd/prestd

$ prestd git:(main) PREST_CACHE=false \
PREST_PG_HOST=localhost  \
PREST_SSL_MODE=disable   \
PREST_JWT_DEFAULT=false  \
PREST_PG_PORT=5432       \
PREST_PG_PASS=prest      \
PREST_PG_USER=prest      \
PREST_DEBUG=true  \
go run main.go
```
