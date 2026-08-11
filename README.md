# pRESTd

[![Unit tests](https://github.com/prest/prest/actions/workflows/test-unit.yml/badge.svg)](https://github.com/prest/prest/actions/workflows/test-unit.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/prest/prest/v2.svg)](https://pkg.go.dev/github.com/prest/prest/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/prest/prest)](https://goreportcard.com/report/github.com/prest/prest)
[![codecov](https://codecov.io/gh/prest/prest/branch/main/graph/badge.svg?token=eVD9urwIEv)](https://codecov.io/gh/prest/prest)
[![Homebrew](https://img.shields.io/badge/dynamic/json.svg?url=https://formulae.brew.sh/api/formula/prestd.json&query=$.versions.stable&label=homebrew)](https://formulae.brew.sh/formula/prestd)
[![Discord](https://img.shields.io/badge/discord-prestd-blue?logo=discord)](https://discord.gg/JnRjvu39w8)

_p_**REST** (**P**_ostgreSQL_ **REST**) is a production-ready API that delivers instant REST and Model Context Protocol (MCP) APIs on top of your **existing or new Postgres** database—CRUD, custom SQL routes, auth, ACL, and a read-only MCP endpoint—without hand-writing a backend.

> PostgreSQL version 9.5 or higher

Contributor License Agreement — [![CLA assistant](https://cla-assistant.io/readme/badge/prest/prest)](https://cla-assistant.io/prest/prest)

<a href="https://www.producthunt.com/posts/prest?utm_source=badge-featured&utm_medium=badge&utm_souce=badge-prest" target="_blank"><img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=303506&theme=light" alt="pREST - instant, realtime, high-performance on PostgreSQL | Product Hunt" style="width: 250px; height: 54px;" width="250" height="54" /></a>

## Documentation

**Full documentation lives at [docs.prestd.com](https://docs.prestd.com/).**

| Topic | Link |
|-------|------|
| Get pREST (Docker, Homebrew, Go) | [Get pREST](https://docs.prestd.com/get-prest) |
| Configuration | [Configuring pREST](https://docs.prestd.com/get-started/configuring-prest) |
| API reference | [API Reference](https://docs.prestd.com/api-reference) |
| MCP over HTTP | [MCP over HTTP](https://docs.prestd.com/get-started/mcp-over-http) |
| Multi-database | [Multi-database](https://docs.prestd.com/get-started/multi-database) |
| Databases & roadmap | [Databases](https://docs.prestd.com/databases) · [Roadmap](https://docs.prestd.com/databases/roadmap) |
| AI clients (Cursor, Claude, …) | [AI and MCP](https://docs.prestd.com/ai) |
## Quick start

Install and run options (Docker, Homebrew, or Go) are documented in [Get pREST](https://docs.prestd.com/get-prest). Point pREST at Postgres (`PREST_PG_URL` or `pg.*` / `DATABASE_URL`), then call:

```http
GET /{database}/{schema}/{table}
```

See [Configuring pREST](https://docs.prestd.com/get-started/configuring-prest) for auth, ACL, custom queries, and MCP.

## Passing values to query scripts

Values a `_QUERIES` script template interpolates become part of the SQL text, so
pREST screens them: anything carrying quotes, `--`, `::`, or — for multi-word
values — a SQL keyword is refused, and a refused value that is interpolated fails
the request with `400`.

Bind free-form values instead, and the screen does not apply at all. A bound value
travels to Postgres out of band, where it can never be parsed as SQL:

```sql
-- interpolated: screened, and rejected for values like 'compra do mes'
SELECT * FROM articles WHERE slug = '{{.slug}}'

-- bound: the caller's value arrives verbatim, whatever it contains
SELECT * FROM articles WHERE slug = {{sqlVal "slug"}}
```

| Helper | Use for | Renders |
|--------|---------|---------|
| `{{sqlVal "key"}}` | a single value | `$1` |
| `{{sqlList "key"}}` | a repeated query parameter (`?tag=a&tag=b`) | `($1,$2)` |
| `{{ident "key"}}` | a table/column name, which cannot be bound | `"public"."users"` |

`sqlVal` and `sqlList` also reach headers as `{{sqlVal "header.X-Application"}}`.
Credential headers (`Authorization`, `Cookie`, …) are always withheld.

Prefer binding for anything user-supplied — search phrases especially, since a
phrase containing a common word such as `do`, `as` or `or` is exactly what the
interpolation screen refuses.

## 1-Click Deploy

### Heroku

Deploy to Heroku and get a realtime RESTful API backed by Heroku Postgres:

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/prest/prest-heroku)

More: [Deploy in Heroku](https://docs.prestd.com/deployment/deploy-in-heroku).

## Contributing

Contributions are welcome. See the [Development Guide](https://docs.prestd.com/get-prest/development-guide) and [Contributing to pREST](https://docs.prestd.com/readme/contributing-to-prestd). Please sign the [CLA](https://cla-assistant.io/prest/prest).

Questions? [GitHub Discussions](https://github.com/prest/prest/discussions) or [Discord](https://discord.gg/JnRjvu39w8).

## Testing

Run unit tests locally:

```bash
make test-unit
```

Run integration suites inside Docker (no local Postgres required):

```bash
# Postgres (full stack: default, auth, multicluster, queries) — also: make test-integration
make test-integration-postgres

# TimescaleDB (Timescale-specific E2E only)
make test-integration-timescaledb
```

Or with Docker Compose:

```bash
docker compose -f integration/postgres/docker-compose.yml up -d --wait \
  postgres postgres-b db-init prestd prestd-multicluster prestd-auth prestd-queries
docker compose -f integration/postgres/docker-compose.yml run --rm --no-deps tests
docker compose -f integration/postgres/docker-compose.yml down -v --remove-orphans
```

Postgres compose runs `./integration/suites/...` and `./integration/postgres/...`.
TimescaleDB compose runs `./integration/timescaledb/...` only (see `.github/workflows/test-integration-timescaledb.yml`).
Network tests require `PREST_TEST_URL` (and flavor-specific URLs for the Postgres job); outside Compose those tests skip when the URLs are unset.

For adapter-dependent unit tests that should not open PostgreSQL connections,
see the [`adapters/mock`](adapters/mock/README.md) test adapter documentation.

## Example: Docker Build

Build the Docker image locally for development (compiles from source):

```bash
docker build -t prest/prest:latest .
```

For release builds, GoReleaser uses the same `Dockerfile` / `Dockerfile.noplugins` with a pre-built `prestd` binary. Local source builds can pass version metadata via build arguments:

```bash
docker build \
  --build-arg VERSION=v1.0.0 \
  --build-arg COMMIT=hash \
  --build-arg DATE=2026-02-11 \
  -t prest/prest:latest .
```
