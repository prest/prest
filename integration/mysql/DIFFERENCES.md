# MySQL 8.0+ vs stock Postgres (pREST)

MySQL is a **new SQL dialect**, not a wire-compatible Postgres variant. pREST selects it with an explicit `engine = "mysql"` setting (root config or a `[[databases]]` entry). Empty `engine` stays Postgres, including Timescale detection. One exception: a `[[databases]]` entry whose `url` starts with `mysql://` and has no `engine` is treated as MySQL. Other connection strings are not probed.

## Supported servers

Floor is **MySQL 8.0**. The upsert row alias (`INSERT ... VALUES (...) AS new ON DUPLICATE KEY UPDATE ... new.col`) needs **8.0.19 or newer**. `VALUES(col)` is not used: it has been deprecated since 8.0.20 and is still only deprecated, not removed, in the 26.7 manual. The published `mysql:8.0` tag (last pushed 2026-05-05) resolves to 8.0.46, which is past 8.0.19. There is no `CREATE INDEX IF NOT EXISTS` (absent on 8.0); a duplicate index is error 1061 and is ignored. Pagination is `LIMIT n OFFSET m` with the offset computed in Go.

As of 2026-09-30 the official Docker library tags are `8.4` → 8.4.11, `9` / `lts` → 9.7.2, and `26.7` / `innovation` / `latest` → 26.7.0. `mysql:8.0` is still pullable but is no longer in that supported-tag list. CI (`.github/workflows/test-integration-mysql.yml`) runs `mysql:8.0`, `mysql:8.4`, and `mysql:latest` (the Docker Library latest tag, which tracks the current innovation release):

| Image | Why |
|-------|-----|
| `mysql:8.0` | Floor. Resolves to 8.0.46, so the 8.0.19 alias syntax is present. |
| `mysql:8.4` | 8.4 LTS line (8.4.11). Default for local `make test-integration-mysql`. |
| `mysql:latest` | Docker Library latest tag, which tracks the current innovation release. On 2026-09-30 it resolved to 26.7.0; the tag moves. |

`9.7` (`lts`) is not a fourth CI image. The SQL this adapter emits is the same statement on 8.0.19 through 26.7: no 8.4-only or 26-only syntax. MariaDB is out of scope.

One `make test-integration-mysql` runs one image so the default target stays a single compose stack. Override with `MYSQL_IMAGE=mysql:8.0 make test-integration-mysql` (or `mysql:8.4`, `mysql:latest`). Compose reads `MYSQL_IMAGE` and defaults to `mysql:8.4`. The workflow matrix runs all three.

Routes stay `/{database}/{schema}/{table}`. `{database}` is the pREST connection alias (or the configured database name on a single connection) and only selects a pool. `{schema}` is the MySQL database — MySQL treats database and schema as the same thing. `{table}` is the table. SQL qualification is always `` `schema`.`table` ``. A path segment never opens a new connection.

| Area | Stock Postgres | MySQL 8.0+ impact on pREST |
|------|----------------|----------------------------|
| Image / init | `postgres:18` | `MYSQL_IMAGE`, default `mysql:8.4`. CI runs `mysql:8.0`, `mysql:8.4`, and `mysql:latest` (see Supported servers). Compose sets a root password and creates the app user in init. No extensions. MariaDB is out of scope (it has `RETURNING`; MySQL 8.0 through 26.7 does not). |
| Adapter selection | `adapters/postgres`, Timescale auto-detect | `adapters/mysql` only when `engine=mysql`. Unknown engine fails startup. |
| Connection / DSN / driver | `lib/pq`, `postgres://` or keyword DSN | `github.com/go-sql-driver/mysql`. DSN `user:pass@tcp(host:port)/dbname?parseTime=true&charset=utf8mb4&multiStatements=false&interpolateParams=true` plus `tls`. Default is `interpolateParams=true` (no prepare per statement). `mysql.prepare = true` sets `interpolateParams=false` and uses the binary prepare protocol. `multiStatements=false` is unchanged. `mysql://` is parsed only for `engine=mysql`. Missing user or database fails startup. Postgres defaults (`127.0.0.1:5432`, user `postgres`, database `prest`) are not copied. Port defaults to `3306` only when unset. |
| TLS | `sslmode` plus client cert files | `disable` → `tls=false`, `require` → `tls=true`, `skip-verify` → `tls=skip-verify`. Cert, key, or rootcert set fails startup (custom TLS config is a follow-up). |
| Identifier quoting | `"ident"` | Backticks, embedded backticks doubled. Placeholders are `?` in argument order. No three-part names. |
| Catalog | `pg_database` / `pg_namespace` / `pg_class` | `information_schema`. System schemas `mysql`, `information_schema`, `performance_schema`, and `sys` are hidden. `/databases` and `/schemas` both list user databases (a schema is a database). Relation kinds are only `table` and `view`. `owner` is null. There is no `datistemplate`. |
| Show columns | `information_schema.columns` including `is_generated` / `is_updatable` | Same JSON keys. `is_generated` is `ALWAYS` when `EXTRA` is `VIRTUAL GENERATED` or `STORED GENERATED`, otherwise `NEVER`. `is_updatable` is the literal `YES`. |
| JSON results | `json_agg` / `row_to_json` | The statement runs as written. Rows are scanned in Go into one JSON array. `NULL` → `null`, integers and float/double → numbers, `DECIMAL`/`NUMERIC` → string, date/time (`parseTime=true`) → RFC3339, valid JSON columns → raw JSON, other bytes → string. Empty result → `[]`. |
| Operators | `$eq` `$ilike` `$any` arrays, ltree, tsquery, pgvector | `$eq` `$ne` `$gt` `$gte` `$lt` `$lte` `$in` `$nin` `$like` `$nlike` `$null` `$notnull` `$true` `$nottrue` `$false` `$notfalse` match Postgres tokens. `$ilike` / `$nilike` become `LIKE` / `NOT LIKE` (case follows the column collation, typically `utf8mb4_0900_ai_ci`; pREST does not inject `LOWER()`). `$any` / `$some` expand to `IN (?,?,?)`. `$all`, ltree, `$tsquery`, and `vecdist` error with `unsupported operator`. JSON `col:jsonb` with `field->>attr` is `` `col`->>'$.attr' ``. Pagination is `LIMIT n OFFSET m` with the offset computed in Go; MySQL rejects `OFFSET(page-1)*size`. |
| Joins | including `FULL` | `INNER` / `LEFT` / `RIGHT` / `CROSS` with backticks. `FULL` errors (no `FULL OUTER JOIN`). |
| Writes | `RETURNING`, `COPY` | `INSERT` runs, then `SELECT *` in the same transaction using primary-key values. `AUTO_INCREMENT` binds `LastInsertId()`. If a primary-key column is neither in the payload nor auto-increment, the follow-up `SELECT` is skipped and the JSON object is built from bound columns (defaults and triggers are then absent). `UPDATE`/`DELETE` without `_returning` return `{"rows_affected": n}`. With `_returning`, the controller suffix is stripped and not sent to MySQL: `UPDATE` selects the post-image, `DELETE` selects the pre-image then deletes, both in one `REPEATABLE READ` transaction. |
| Batch / copy | multi-row `INSERT` or `COPY` | `BatchInsertValues` is a multi-row `INSERT`. `Prest-Batch-Method: copy` runs that same insert inside a transaction. It is not `LOAD DATA`. |
| Missing table | `pq: relation ... does not exist` → 404 | MySQL error 1146 is wrapped as `adapters.ErrRelationNotFound` and the CRUD handler returns 404. |
| DDL / engine features | `RETURNING`, arrays, `COPY`, ltree, tsquery, pgvector, `time_bucket`, hypertables | None of those are emitted. `TimeBucketClause` is empty. Slices and objects in JSON bodies are bound as JSON text (a Go `string`, so utf8mb4), not Postgres array literals. They are not bound as `[]byte`, because with `interpolateParams=true` the driver sends `[]byte` as a `_binary` literal, which JSON columns reject (error 3144). Binding is the same with `prepare` on or off, so `[[databases]]` entries need no per-entry `prepare` key. |
| Scripts | `$n` placeholders, `"ident"` | Filesystem layout is unchanged. MySQL templates use `?` and backtick `ident`. `inFormat` still interpolates quoted literals; prefer `sqlList`. |
| System tables | `serial`, `jsonb`, `TIMESTAMPTZ`, `ON CONFLICT` | Auth: `` `id` BIGINT AUTO_INCREMENT PRIMARY KEY ``, `name`/`username`/`password` `TEXT`, unique `username` via `UNIQUE KEY username (username(255))` (MySQL rejects a full unique key on `TEXT`), `metadata` `JSON`. `prest_queries`: `BIGINT AUTO_INCREMENT`, identity columns `database_alias`/`location`/`name` are `VARCHAR(255)` so `UNIQUE KEY identity` needs no prefix, verb columns `TEXT`, `DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)`, `updated_at` `ON UPDATE CURRENT_TIMESTAMP(6)`. No `CREATE INDEX IF NOT EXISTS` (duplicate index error 1061 is ignored). Upsert is `INSERT ... VALUES (...) AS new ON DUPLICATE KEY UPDATE` (MySQL 8.0.19+; same statement on 8.4 and 26.7). `created_at::text` is `CAST(created_at AS CHAR)`. `now()` is `CURRENT_TIMESTAMP(6)`. |
| Auth | `"schema"."table"`, `$1`/`$2` | `/auth` builds its user lookup through the adapter's `Dialect`: backticks and `?`. `auth.schema` defaults to the connection's database when unset, since `public` does not exist on MySQL. |
| MCP (`/_mcp`) | `"ident"`, `$n` | Supported. `select_table` quotes with backticks and binds filters as `?`. In mixed setups each alias uses its own adapter's catalog, executor, and dialect. |
| `_count` | Column is named `count` implicitly | `COUNT(...) AS `+"`count`"+`, so the response key matches Postgres. |
| ACL | TOML allowlist | Same TOML allowlist (`access.restrict` and per-table rules). System schemas are excluded from listing; they are not a substitute for `access.restrict`. |
| Compose ownership | Auth, multicluster, queries, shared `integration/suites` | This compose is MySQL-only. It runs auth (`prestd-auth` on port 3002 with a seeded bcrypt user), MCP, and queries registry tests too (`prestd-queries` on port 3003 with `testdata/prest_queries.toml`, `PREST_AUTH_SCHEMA=shop`, and a seeded md5 admin and `test7` table). Multicluster and `integration/suites` stay on the Postgres workflow. `make test-integration-mysql` runs `./integration/mysql/...` only, against one `MYSQL_IMAGE`. The MySQL workflow matrix runs that same package on `mysql:8.0`, `mysql:8.4`, and `mysql:latest`. |

## Shared suites vs MySQL E2E

Wire-compatible shared suites live under `integration/suites/` and run on the **Postgres** integration workflow (`make test-integration-postgres`).

The MySQL workflow (`test-integration-mysql.yml` / `make test-integration-mysql`) runs **only** `./integration/mysql/...` against this compose.
