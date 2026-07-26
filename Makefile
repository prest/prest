DOCKER_COMPOSE?=docker-compose -f docker-compose.yml
UNIT_PKGS = $(shell go list ./... | grep -v '/integration')
RACE?=-race

.PHONY: build test test-unit test-race vet lint ci \
        test-integration test-integration-postgres test-integration-timescaledb \
        test-integration-log test-integration-postgres-log test-integration-timescaledb-log \
        build_test_image signoz-up signoz-down \
        dc-up dc-down mockgen \
        studio-install studio-dev studio-format studio-lint studio-typecheck \
        studio-test studio-test-coverage studio-build studio-check studio-e2e test-studio check-all

# ──────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────

build:
	go build -o bin/prestd ./cmd/prestd

build-test-image:
	$(DOCKER_COMPOSE) up -d postgres

# ──────────────────────────────────────────────
# Code quality
# ──────────────────────────────────────────────

vet:
	go vet ./...

lint: vet test-unit

# ──────────────────────────────────────────────
# Unit tests
# ──────────────────────────────────────────────

test: test-unit

test-unit:
	go test -timeout 60s -tags prest_test_hooks -count=1 -covermode=atomic -coverprofile=coverage.out $(UNIT_PKGS)

test-race:
	go test -timeout 60s -tags prest_test_hooks $(RACE) -count=1 -covermode=atomic -coverprofile=coverage.out $(UNIT_PKGS)

# ──────────────────────────────────────────────
# Integration tests (require Docker)
# ──────────────────────────────────────────────

POSTGRES_COMPOSE=docker compose -f integration/postgres/docker-compose.yml
TIMESCALEDB_COMPOSE=docker compose -f integration/timescaledb/docker-compose.yml

test-integration: test-integration-postgres

test-integration-postgres:
	$(POSTGRES_COMPOSE) up -d --wait postgres postgres-b db-init prestd prestd-multicluster prestd-auth prestd-queries && \
	$(POSTGRES_COMPOSE) run --rm --no-deps tests; \
	status=$$?; \
	$(POSTGRES_COMPOSE) down -v --remove-orphans; \
	exit $$status

test-integration-timescaledb:
	$(TIMESCALEDB_COMPOSE) up -d --wait timescaledb db-init prestd && \
	$(TIMESCALEDB_COMPOSE) run --rm --no-deps tests; \
	status=$$?; \
	$(TIMESCALEDB_COMPOSE) down -v --remove-orphans; \
	exit $$status

INTEGRATION_LOG ?= integration-test.log
TEE ?= tee

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

ci: test-integration-postgres test-integration-timescaledb

# ──────────────────────────────────────────────
# SigNoz (OpenTelemetry observability)
# ──────────────────────────────────────────────

SIGNOZ_COMPOSE=docker compose -f dev/signoz/docker-compose.yaml

signoz-up:
	$(SIGNOZ_COMPOSE) up -d

signoz-down:
	$(SIGNOZ_COMPOSE) down -v --remove-orphans

# ──────────────────────────────────────────────
# Docker
# ──────────────────────────────────────────────

dc-up:
	$(DOCKER_COMPOSE) up --force-recreate --remove-orphans --build

dc-down:
	$(DOCKER_COMPOSE) down --volumes --remove-orphans --rmi local

# ──────────────────────────────────────────────
# Mockgen (regenerate adapter mocks)
# ──────────────────────────────────────────────

mockgen:
	go install github.com/golang/mock/mockgen@v1.6.0
	mockgen -destination=internal/mockgen/scanner.go -package=mockgen github.com/prest/prest/v2/pkg/adapters Scanner
	mockgen -destination=internal/mockgen/adapter.go -package=mockgen github.com/prest/prest/v2/pkg/adapters Adapter
	mockgen -destination=internal/mockgen/request_query_builder.go -package=mockgen github.com/prest/prest/v2/pkg/adapters RequestQueryBuilder
	mockgen -destination=internal/mockgen/query_executor.go -package=mockgen github.com/prest/prest/v2/pkg/adapters QueryExecutor
	mockgen -destination=internal/mockgen/catalog_querier.go -package=mockgen github.com/prest/prest/v2/pkg/adapters CatalogQuerier
	mockgen -destination=internal/mockgen/sql_builder.go -package=mockgen github.com/prest/prest/v2/pkg/adapters SQLBuilder
	mockgen -destination=internal/mockgen/permissions_checker.go -package=mockgen github.com/prest/prest/v2/pkg/adapters PermissionsChecker
	mockgen -destination=internal/mockgen/script_runner.go -package=mockgen github.com/prest/prest/v2/pkg/adapters ScriptRunner
	mockgen -destination=internal/mockgen/query_registry.go -package=mockgen github.com/prest/prest/v2/pkg/adapters QueryRegistry
	mockgen -destination=internal/mockgen/script_permissions_checker.go -package=mockgen github.com/prest/prest/v2/pkg/adapters ScriptPermissionsChecker
	mockgen -destination=internal/mockgen/database_registry.go -package=mockgen github.com/prest/prest/v2/pkg/adapters DatabaseRegistry
	mockgen -destination=internal/mockgen/database_pinger.go -package=mockgen github.com/prest/prest/v2/pkg/adapters DatabasePinger
	mockgen -destination=internal/mockgen/readiness_checker.go -package=mockgen github.com/prest/prest/v2/pkg/adapters ReadinessChecker

# ──────────────────────────────────────────────
# Studio (frontend)
# ──────────────────────────────────────────────

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

# ──────────────────────────────────────────────
# All checks
# ──────────────────────────────────────────────

check-all: vet test-unit studio-check
