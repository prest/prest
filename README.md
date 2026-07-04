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

## Multi-database

pREST uses the first URL path segment as the **database selector** for CRUD, catalog, and optional script routes. Two modes are supported:

| Mode | When | `{database}` in URL | Connection target |
|------|------|---------------------|-------------------|
| **Legacy multi-DB** | No registry configured | Postgres database name | Same `pg.host`; `dbname` = path segment |
| **Registry multi-cluster** | `[[databases]]` or env registry set | Registered **alias** | Per-profile host, port, and credentials |

### URL routing

All table operations use `/{database}/{schema}/{table}`:

```http
GET /tenant-a/public/users
POST /tenant-a/public/orders
GET /tenant-a/public
GET /_QUERIES/tenant-a/myqueries/get_all
```

Script routes accept an optional database prefix (`/_QUERIES/{database}/{queriesLocation}/{script}`). When omitted, the default database (`pg.database`) is used.

Request flow: validate alias → set connection context → open or reuse pool for that alias → execute query.

### Configuration

Registry sources are merged in priority order: **indexed env pairs → TOML** (env wins on conflict).

#### Environment variables (production / Kubernetes)

Register databases with contiguous 1-based index pairs:

```sh
DATABASE_ALIAS_1=tenant-a
DATABASE_URL_1=postgres://user:pass@cluster-a.example.com:5432/app_a?sslmode=require
DATABASE_ALIAS_2=tenant-b
DATABASE_URL_2=postgres://user:pass@cluster-b.example.com:5432/app_b?sslmode=require
```

`PREST_DATABASE_ALIAS_N` and `PREST_DATABASE_URL_N` are accepted as aliases of the above keys.

See [`install-manifests/kubernetes/deployment.yaml`](install-manifests/kubernetes/deployment.yaml) for a multi-secret example with liveness/readiness probes.

#### TOML (local development)

`pg.*` remains the default/fallback profile; registry entries override host, port, and credentials per alias:

```toml
[pg]
database = "prest-test"
single = false

[[databases]]
alias = "prest-test"
host = "postgres"
port = 5432
database = "prest-test"
user = "postgres"
pass = "postgres"
ssl.mode = "disable"

[[databases]]
alias = "secondary-db"
host = "postgres-b"
port = 5432
database = "secondary-cluster"
user = "postgres"
pass = "postgres"
ssl.mode = "disable"
```

When no registry is configured, legacy `DATABASE_URL` / `pg.*` behavior is unchanged.

### Alias vs physical database name

- URLs and access rules use the **alias** (e.g. `tenant-a`).
- Connection pools use the profile's `database`, `host`, and credentials (e.g. `app_a` on `cluster-a.example.com`).
- When alias equals the physical database name (legacy mode), behavior matches pre-registry pREST.

### `pg.single`

Set `pg.single = false` to allow routing to multiple databases or aliases. When `true` and a registry is active, only the default database alias is accepted.

### Connection pooling

Pools are keyed by connection URI; aliases that share the same URI share a pool. Connections are opened lazily on first request per alias.

**Connection budgeting:** plan for `replicas × aliases × pg.maxopenconn` connections per cluster. Use PgBouncer or RDS Proxy when many aliases are registered.

### Health checks

| Endpoint | Purpose | Behavior |
|----------|---------|----------|
| `GET /_health` | Liveness | Pings the default database |
| `GET /_ready` | Readiness | Pings the default database and every registered alias |

### Access control

`access.tables` entries support an optional `database` field for per-alias permissions:

```toml
[[access.tables]]
database = "tenant-a"
schema = "public"
name = "users"
permissions = ["read"]
```

### Local testing

Multi-cluster integration tests live in [`integration/controllers/multicluster_test.go`](integration/controllers/multicluster_test.go). They require a second Postgres service (`PREST_PG_HOST_B`) provided by [`docker-compose-test.yml`](docker-compose-test.yml):

```bash
make test-integration
```

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

## Example: Docker Build

You can build the Docker image locally for development (this compiles the code from source):

```bash
docker build -t prest/prest:latest .
```

For release builds, GoReleaser uses the same `Dockerfile` / `Dockerfile.noplugins` with a pre-built `prestd` binary (no `go.mod` in the build context). Local source builds pass version metadata via build arguments:

```bash
docker build \
  --build-arg VERSION=v1.0.0 \
  --build-arg COMMIT=hash \
  --build-arg DATE=2026-02-11 \
  -t prest/prest:latest .
```

