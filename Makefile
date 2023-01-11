DOCKER_COMPOSE?=docker-compose

PHONY: \
	build_test_image \

build_test_image:
	$(DOCKER_COMPOSE) -f docker-compose.yml  run --rm postgres -d

test:
	$(DOCKER_COMPOSE) -f testdata/docker-compose.yml up --abort-on-container-exit --remove-orphans
