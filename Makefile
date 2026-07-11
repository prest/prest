DOCKER_COMPOSE?=docker-compose -f docker-compose.yml
UNIT_PKGS = $(shell go list ./... | grep -v '/integration')

.PHONY: build_test_image test test-unit test-integration
build_test_image:
	$(DOCKER_COMPOSE) up -d postgres

test: test-unit

test-unit:
	go test -timeout 30s -tags prest_test_hooks -race -count=1 -covermode=atomic -coverprofile=coverage.out $(UNIT_PKGS)

test-integration:
	docker compose -f docker-compose-test.yml up -d --wait postgres postgres-b db-init prestd prestd-multicluster prestd-auth prestd-queries && \
	docker compose -f docker-compose-test.yml run --rm --no-deps tests; \
	status=$$?; \
	docker compose -f docker-compose-test.yml down -v --remove-orphans; \
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
