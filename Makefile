DOCKER_COMPOSE?=docker-compose -f docker-compose.yml

.PHONY: build_test_image
build_test_image:
	$(DOCKER_COMPOSE) run --rm postgres -d

.PHONY: test
test:
	docker compose -f docker-compose-test.yml up --abort-on-container-exit --exit-code-from tests
	docker compose -f docker-compose-test.yml down -v --remove-orphans

PHONY: dc-up
dc-up:
	$(DOCKER_COMPOSE) up \
		--force-recreate \
		--remove-orphans \
		--build

PHONY: dc-down
dc-down:
	$(DOCKER_COMPOSE) down --volumes --remove-orphans --rmi local

PHONY: mockgen
mockgen:
	go install github.com/golang/mock/mockgen@v1.6.0
	mockgen -destination=adapters/mockgen/scanner.go -package=mockgen github.com/prest/prest/v2/adapters Scanner
	mockgen -destination=adapters/mockgen/adapter.go -package=mockgen github.com/prest/prest/v2/adapters Adapter
