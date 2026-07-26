.PHONY: help up down build logs ps shell restart clean

TESTDATA_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
COMPOSE_FILE := $(TESTDATA_DIR)/docker-compose.yml
POSTGRES_HOST_PORT ?= 5432

# Choose Podman when available, otherwise Docker.
CONTAINER_ENGINE ?= $(shell if command -v podman >/dev/null 2>&1; then echo podman; else echo docker; fi)

# Prefer podman compose, then docker compose, then docker-compose.
COMPOSE_CMD ?= $(shell if [ "$(CONTAINER_ENGINE)" = "podman" ]; then echo "podman compose"; elif command -v docker-compose >/dev/null 2>&1; then echo docker-compose; else echo "docker compose"; fi)

help:
	@echo "Usage:"
	@echo "  make -f testdata/Makefile up        # start postgres + prest-test"
	@echo "  make -f testdata/Makefile down      # stop and remove containers"
	@echo "  make -f testdata/Makefile logs      # follow logs"
	@echo "  make -f testdata/Makefile up POSTGRES_HOST_PORT=15432  # use a custom host port"
	@echo "  make -f testdata/Makefile shell     # open a shell in prest-test"
	@echo "  make -f testdata/Makefile ps        # list containers"
	@echo "  make -f testdata/Makefile restart   # restart services"
	@echo "  make -f testdata/Makefile clean     # remove everything"
	@echo ""
	@echo "Container engine: $(CONTAINER_ENGINE)"
	@echo "Compose command: $(COMPOSE_CMD)"

check:
	@command -v $(CONTAINER_ENGINE) >/dev/null 2>&1 || { echo "$(CONTAINER_ENGINE) not found"; exit 1; }

up: check
	POSTGRES_HOST_PORT=$(POSTGRES_HOST_PORT) $(COMPOSE_CMD) -f $(COMPOSE_FILE) up -d --build

build: check
	POSTGRES_HOST_PORT=$(POSTGRES_HOST_PORT) $(COMPOSE_CMD) -f $(COMPOSE_FILE) build

down: check
	POSTGRES_HOST_PORT=$(POSTGRES_HOST_PORT) $(COMPOSE_CMD) -f $(COMPOSE_FILE) down --volumes --remove-orphans

logs: check
	POSTGRES_HOST_PORT=$(POSTGRES_HOST_PORT) $(COMPOSE_CMD) -f $(COMPOSE_FILE) logs -f

ps: check
	POSTGRES_HOST_PORT=$(POSTGRES_HOST_PORT) $(COMPOSE_CMD) -f $(COMPOSE_FILE) ps

shell: check
	$(CONTAINER_ENGINE) exec -it prest-test /bin/sh

restart: check
	POSTGRES_HOST_PORT=$(POSTGRES_HOST_PORT) $(COMPOSE_CMD) -f $(COMPOSE_FILE) restart

clean: down
	@echo "Compose environment removed"
