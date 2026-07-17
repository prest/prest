# pREST Studio

Admin & explorer UI for [pREST](https://github.com/prest/prest), embedded in the
`prestd` binary and served at **`/_studio/`**.

Studio is a static single-page app (React 19 + Vite + Tailwind v4). The
production build is written into `../internal/studio/dist`, where it is embedded
via `go:embed` and served by `internal/studio/handler.go`.

## Features

- **Overview** — server health/readiness, MCP availability, catalog counts,
  build metadata and bearer-token status, with quick links.
- **Data Explorer** — schema/table tree, table structure, paginated rows
  (page size 25), a filter builder, and copy-as-URL / copy-as-`curl`. Selection
  is reflected in URL search params. Studio is **read-only** (GET only).
- **REST Explorer** — a `GET` request builder with query parameters and a
  response panel.
- **MCP Explorer** — connect to `/_mcp`, list/search tools, invoke them via a
  generated form or raw JSON, keep an invocation history, and copy a ready-made
  "Connect an AI client" configuration.

## Requirements

- Node `>= 24` (see `.node-version`)
- pnpm (via Corepack: `corepack enable`)

## Getting started

```sh
corepack enable
pnpm install

# Dev server on http://localhost:5173/_studio/ (proxies the API to prestd).
# Point it at your running prestd with VITE_PREST_PROXY_TARGET (default :3000).
pnpm dev
```

Copy `.env.example` to `.env` to override the proxy target.

## Scripts

| Script                              | Description                                                       |
| ----------------------------------- | ----------------------------------------------------------------- |
| `pnpm dev`                          | Vite dev server with API proxy                                    |
| `pnpm build`                        | Type-check + build into `../internal/studio/dist`                 |
| `pnpm preview`                      | Preview the production build                                      |
| `pnpm typecheck`                    | `tsc --noEmit`                                                    |
| `pnpm lint` / `pnpm lint:fix`       | ESLint                                                            |
| `pnpm format` / `pnpm format:check` | Prettier                                                          |
| `pnpm test` / `pnpm test:coverage`  | Vitest (unit)                                                     |
| `pnpm test:e2e`                     | Playwright smoke tests                                            |
| `pnpm check`                        | `format:check` + `lint` + `typecheck` + `test:coverage` + `build` |

## Architecture

```
src/
  app/          providers (QueryClient, Theme, Auth, PrestClient) + router
  components/
    ui/         Radix-based primitives (Button, Dialog, Input, Label, Badge, Card)
    layout/     AppShell, Sidebar, MobileNav, AuthDialog, ThemeToggle
  features/     overview, catalog (data), rest-explorer, mcp
  lib/
    api/        typed pREST fetch client, URL/query builders, curl, endpoint helpers
    auth/       bearer-token store (memory or per-tab sessionStorage)
    mcp/        JSON-RPC 2.0 client for /_mcp
    errors.ts   typed ApiError hierarchy
    utils.ts    cn() class-name helper
```

### Testing & coverage

Unit tests focus on the framework-agnostic logic under `src/lib/**` (API/MCP
clients, URL/curl builders, auth store, error mapping). Coverage thresholds are
enforced there (see `vite.config.ts`). Run:

```sh
pnpm test:coverage
```

Playwright smoke tests live in `e2e/`. Backend-dependent specs are skipped
unless `PREST_TEST_URL` is set.

## Security notes

- The bearer token is **never** written to `localStorage`. It lives in memory
  by default, or in `sessionStorage` (per tab) when "remember" is enabled.
- Generated `curl` snippets omit the `Authorization` header by default.
- The "Connect an AI client" snippet uses a `<YOUR_TOKEN>` placeholder rather
  than embedding the live token.
