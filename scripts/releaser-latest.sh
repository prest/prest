#!/usr/bin/env bash
docker build . -t prest/prest:latest && \
docker login -u="$DOCKER_LOGIN" -p="$DOCKER_PASSWORD" && \
docker push prest/prest:latest
