DOCKER_COMPOSE?=docker-compose -f docker-compose.yml
UNIT_PKGS = $(shell go list ./... | grep -v '/integration')

.PHONY: build_test_image test test-unit test-integration test-integration-postgres test-integration-timescaledb test-integration-log test-integration-postgres-log test-integration-timescaledb-log ci signoz-up signoz-down
build_test_image:
	$(DOCKER_COMPOSE) up -d postgres

SIGNOZ_COMPOSE=docker compose -f dev/signoz/docker-compose.yaml

# Local SigNoz stack to view pREST OpenTelemetry signals (traces/metrics/logs).
# After `make signoz-up`, run prestd with:
#   PREST_OTEL_ENABLED=true PREST_OTEL_ENDPOINT=localhost:4317 PREST_OTEL_INSECURE=true
# and open the SigNoz UI at http://localhost:8080
signoz-up:
	$(SIGNOZ_COMPOSE) up -d

signoz-down:
	$(SIGNOZ_COMPOSE) down -v --remove-orphans

ci: test-integration-postgres test-integration-timescaledb

test: test-unit

test-unit:
	go test -timeout 30s -tags prest_test_hooks -race -count=1 -covermode=atomic -coverprofile=coverage.out $(UNIT_PKGS)

POSTGRES_COMPOSE=docker compose -f integration/postgres/docker-compose.yml
TIMESCALEDB_COMPOSE=docker compose -f integration/timescaledb/docker-compose.yml

# Alias for the historical Postgres integration target.
test-integration: test-integration-postgres

test-integration-postgres:
	$(POSTGRES_COMPOSE) up -d --wait postgres postgres-b db-init prestd prestd-multicluster prestd-auth prestd-queries && \
	$(POSTGRES_COMPOSE) run --rm --no-deps tests; \
	status=$$?; \
	$(POSTGRES_COMPOSE) down -v --remove-orphans; \
	exit $$status

# Same as test-integration but tees the full combined output to a file so the
# whole result can be explored after the run (terminals truncate long output).
# Everything is captured into a single file. The aggregate truncates it once and
# both suites append (TEE='tee -a'), so a failure in the first never overwrites
# or hides the second. A standalone *-log target starts the file fresh.
# Override the destination with: make test-integration-log INTEGRATION_LOG=path.log
INTEGRATION_LOG ?= integration-test.log
TEE ?= tee

# Runs both suites regardless of the first's result, appending to one file, and
# exits non-zero if either failed, so the full output is always available.
test-integration-log:
	@: > $(INTEGRATION_LOG)
	@rc=0; \
	$(MAKE) test-integration-postgres-log TEE='tee -a' || rc=1; \
	$(MAKE) test-integration-timescaledb-log TEE='tee -a' || rc=1; \
	echo "Full output saved to $(INTEGRATION_LOG) (exit $$rc)"; \
	exit $$rc

test-integration-postgres-log:
	@echo "Writing full Postgres integration output to $(INTEGRATION_LOG)"
	@{ \
	  $(POSTGRES_COMPOSE) up -d --wait postgres postgres-b db-init prestd prestd-multicluster prestd-auth prestd-queries && \
	  $(POSTGRES_COMPOSE) run --rm --no-deps tests; \
	  echo $$? > .integration-status.$$$$; \
	  $(POSTGRES_COMPOSE) down -v --remove-orphans; \
	} 2>&1 | $(TEE) $(INTEGRATION_LOG); \
	status=$$(cat .integration-status.$$$$); rm -f .integration-status.$$$$; \
	echo "Full output saved to $(INTEGRATION_LOG) (exit $$status)"; \
	exit $$status

test-integration-timescaledb-log:
	@echo "Writing full TimescaleDB integration output to $(INTEGRATION_LOG)"
	@{ \
	  $(TIMESCALEDB_COMPOSE) up -d --wait timescaledb db-init prestd && \
	  $(TIMESCALEDB_COMPOSE) run --rm --no-deps tests; \
	  echo $$? > .integration-status.$$$$; \
	  $(TIMESCALEDB_COMPOSE) down -v --remove-orphans; \
	} 2>&1 | $(TEE) $(INTEGRATION_LOG); \
	status=$$(cat .integration-status.$$$$); rm -f .integration-status.$$$$; \
	echo "Full output saved to $(INTEGRATION_LOG) (exit $$status)"; \
	exit $$status

test-integration-timescaledb:
	$(TIMESCALEDB_COMPOSE) up -d --wait timescaledb db-init prestd && \
	$(TIMESCALEDB_COMPOSE) run --rm --no-deps tests; \
	status=$$?; \
	$(TIMESCALEDB_COMPOSE) down -v --remove-orphans; \
	exit $$status

.PHONY: dc-up
dc-up:
	$(DOCKER_COMPOSE) up \
		--force-recreate \
		--remove-orphans \
		--build

.PHONY: dc-down
dc-down:
	$(DOCKER_COMPOSE) down --volumes --remove-orphans --rmi local

.PHONY: mockgen
mockgen:
	go install github.com/golang/mock/mockgen@v1.6.0
	mockgen -destination=adapters/mockgen/scanner.go -package=mockgen github.com/prest/prest/v2/adapters Scanner
	mockgen -destination=adapters/mockgen/adapter.go -package=mockgen github.com/prest/prest/v2/adapters Adapter
	mockgen -destination=adapters/mockgen/request_query_builder.go -package=mockgen github.com/prest/prest/v2/adapters RequestQueryBuilder
	mockgen -destination=adapters/mockgen/query_executor.go -package=mockgen github.com/prest/prest/v2/adapters QueryExecutor
	mockgen -destination=adapters/mockgen/catalog_querier.go -package=mockgen github.com/prest/prest/v2/adapters CatalogQuerier
	mockgen -destination=adapters/mockgen/sql_builder.go -package=mockgen github.com/prest/prest/v2/adapters SQLBuilder
	mockgen -destination=adapters/mockgen/permissions_checker.go -package=mockgen github.com/prest/prest/v2/adapters PermissionsChecker
	mockgen -destination=adapters/mockgen/script_runner.go -package=mockgen github.com/prest/prest/v2/adapters ScriptRunner
	mockgen -destination=adapters/mockgen/query_registry.go -package=mockgen github.com/prest/prest/v2/adapters QueryRegistry
	mockgen -destination=adapters/mockgen/script_permissions_checker.go -package=mockgen github.com/prest/prest/v2/adapters ScriptPermissionsChecker
	mockgen -destination=adapters/mockgen/database_registry.go -package=mockgen github.com/prest/prest/v2/adapters DatabaseRegistry
	mockgen -destination=adapters/mockgen/database_pinger.go -package=mockgen github.com/prest/prest/v2/adapters DatabasePinger
	mockgen -destination=adapters/mockgen/readiness_checker.go -package=mockgen github.com/prest/prest/v2/adapters ReadinessChecker

.PHONY: studio-install studio-dev studio-format studio-lint studio-typecheck studio-test studio-test-coverage studio-build studio-check studio-e2e test-studio check-all
studio-install:
	cd studio && corepack enable && pnpm install

studio-dev:
	cd studio && pnpm dev

studio-format:
	cd studio && pnpm format

studio-lint:
	cd studio && pnpm lint

studio-typecheck:
	cd studio && pnpm typecheck

studio-test:
	cd studio && pnpm test

studio-test-coverage:
	cd studio && pnpm test:coverage

studio-build:
	cd studio && pnpm build

studio-check:
	cd studio && pnpm check

studio-e2e:
	cd studio && pnpm test:e2e

test-studio: studio-check

check-all: test-unit studio-check
