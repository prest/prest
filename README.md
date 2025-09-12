# pRESTd

[![Tests](https://github.com/prest/prest/actions/workflows/test.yml/badge.svg)](https://github.com/prest/prest/actions/workflows/test.yml)
[![GoDoc](https://godoc.org/github.com/prest/prest?status.png)](https://godoc.org/github.com/prest/prest)
[![Go Report Card](https://goreportcard.com/badge/github.com/prest/prest)](https://goreportcard.com/report/github.com/prest/prest)
[![codecov](https://codecov.io/gh/prest/prest/branch/main/graph/badge.svg?token=eVD9urwIEv)](https://codecov.io/gh/prest/prest)
[![Homebrew](https://img.shields.io/badge/dynamic/json.svg?url=https://formulae.brew.sh/api/formula/prestd.json&query=$.versions.stable&label=homebrew)](https://formulae.brew.sh/formula/prestd)
[![Discord](https://img.shields.io/badge/discord-prestd-blue?logo=discord)](https://discord.gg/JnRjvu39w8)

_p_**REST** (**P**_ostgreSQL_ **REST**), is a simple production-ready API, that delivers an instant, realtime, and high-performance application on top of your **existing or new Postgres** database.

> PostgreSQL version 9.5 or higher

Contributor License Agreement - [![CLA assistant](https://cla-assistant.io/readme/badge/prest/prest)](https://cla-assistant.io/prest/prest)

<a href="https://www.producthunt.com/posts/prest?utm_source=badge-featured&utm_medium=badge&utm_souce=badge-prest" target="_blank"><img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=303506&theme=light" alt="pREST - instant, realtime, high-performance on PostgreSQL | Product Hunt" style="width: 250px; height: 54px;" width="250" height="54" /></a>

## Problems we solve

The pREST project is the API that addresses the need for fast and efficient solution in building RESTful APIs on PostgreSQL databases. It simplifies API development by offering:

1. A **lightweight server** with easy configuration;
2. Direct **SQL queries with templating** in customizable URLs;
3. Optimizations for **high performance**;
4. **Enhanced** developer **productivity**;
5. **Authentication and authorization** features;
6. **Pluggable** custom routes and middlewares.

Overall, pREST simplifies the process of creating secure and performant RESTful APIs on top of your new or old PostgreSQL database.

[Read more](https://github.com/prest/prest/issues/41).

## Why we built pREST

When we built pREST, we originally intended to contribute and build with the PostgREST project, although it took a lot of work as the project is in Haskell. At the time, we did not have anything similar or intended to keep working with that tech stack. We've been building production-ready Go applications for a long time, so building a similar project with Golang as its core was natural.

Additionally, as Go has taken a huge role in many other vital projects such as Kubernetes and Docker, and we've been able to use the pREST project in many different companies with success over the years, it has shown to be an excellent decision.

## 1-Click Deploy

### Heroku

Deploy to Heroku and instantly get a realtime RESTFul API backed by Heroku Postgres:

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/prest/prest-heroku)

## Documentation

Visit <https://docs.prestd.com/>

## Testing

Run the test suite inside Docker (no local Postgres required):

```bash
make test
```

Or directly with Docker Compose:

```bash
docker compose -f docker-compose-test.yml up --abort-on-container-exit --exit-code-from tests
docker compose -f docker-compose-test.yml down -v --remove-orphans
```

The `tests` service runs `./testdata/runtest.sh`, provisioning databases and executing Go tests.
