DOCKER_COMPOSE?=docker-compose -f docker-compose.yml

PHONY: build_test_image 
build_test_image:
	$(DOCKER_COMPOSE) run --rm postgres -d

PHONY: unit_test
unit_test:
	go test -v -cover ./...

PHONY: test
test:
	$(DOCKER_COMPOSE) -f testdata/docker-compose.yml up --abort-on-container-exit --remove-orphans

PHONY: dc-up
dc-up:
	$(DOCKER_COMPOSE) up \
		--force-recreate \
		--remove-orphans \
		--build

PHONY: dc-up
dc-down:
	$(DOCKER_COMPOSE) down --volumes --remove-orphans --rmi local

PHONY: mockgen
mockgen:
	go install github.com/golang/mock/mockgen@v1.6.0
	mockgen -source=adapters/scanner/scanner.go -destination=adapters/mockgen/scanner.go -package=mockgen
	mockgen -source=adapters/adapter.go -destination=adapters/mockgen/adapter.go -package=mockgen
	mockgen -source=controllers/config.go -destination=controllers/mockgen/server.go -package=mockgen
	mockgen -source=plugins/loader.go -destination=plugins/mockgen/loader.go -package=mockgen
	mockgen -source=cache/cache.go -destination=cache/mockgen/cache.go -package=mockgen
