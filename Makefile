DOCKER_COMPOSE?=docker-compose

PHONY: \
	build_test_image \

build_test_image:
	$(DOCKER_COMPOSE) -f docker-compose.yml  run --rm postgres -d
