# Local SigNoz — view pREST OpenTelemetry signals

This is a self-contained [SigNoz](https://signoz.io) stack for local development,
so you can see pREST's OpenTelemetry **traces**, **metrics**, and **logs** in a UI.
It is vendored/trimmed from SigNoz `v0.111.0` (`deploy/docker`), with the shared
config files moved under [`common/`](./common). pREST pushes OTLP to the bundled
collector — there is **no scrape endpoint** on pREST itself.

## What runs

| Service | Purpose | Exposed port |
|---|---|---|
| `signoz` | Query service + UI | `8080` (UI) |
| `otel-collector` | OTLP receiver → ClickHouse | `4317` (gRPC), `4318` (HTTP) |
| `clickhouse`, `zookeeper-1`, `schema-migrator-*`, `init-clickhouse` | Storage + schema | internal |

> The first `up` runs `init-clickhouse`, which downloads a small helper binary
> from GitHub — it needs outbound network access once.

## Run it

```sh
make signoz-up          # or: docker compose -f dev/signoz/docker-compose.yaml up -d
```

Wait ~30–60s for `signoz` to become healthy, then open the UI:
http://localhost:8080 (create the initial admin account on first visit).

## Point pREST at the collector

Run `prestd` locally (outside the SigNoz compose) with telemetry enabled:

```sh
PREST_OTEL_ENABLED=true \
PREST_OTEL_ENDPOINT=localhost:4317 \
PREST_OTEL_INSECURE=true \
PREST_OTEL_SERVICE_NAME=prestd \
./prestd
```

Then generate traffic (e.g. `curl http://localhost:3000/<db>/public/<table>` or a
`/_QUERIES/...` call) and explore in SigNoz:

- **Services / Traces** — HTTP server spans named by route template
  (`/{database}/{schema}/{table}`) with child Postgres spans tagged by DB alias.
- **Metrics** — HTTP and DB pool metrics.
- **Logs** — pREST's `slog` output, bridged to OTLP and correlated to the trace.

### Running prest inside the same compose

Uncomment the `PREST_OTEL_*` env and the `signoz-net` network entries in the
repo-root [`docker-compose.yml`](../../docker-compose.yml). Inside the compose
network use the collector's service DNS name instead of localhost:

```
PREST_OTEL_ENDPOINT=signoz-otel-collector:4317
```

## Tear down

```sh
make signoz-down        # docker compose ... down -v --remove-orphans (removes volumes)
```

## Notes

- SigNoz deprecated its official Docker Compose in favor of their Foundry
  installer; this vendored copy is pinned to `v0.111.0` for a stable local demo.
  Override image tags via `VERSION` / `OTELCOL_TAG` env vars.
- This stack is **development-only** — do not use it for production monitoring.
