#!/usr/bin/env bash
docker build . -t prest/prest:v1 && \
docker login -u="$DOCKER_LOGIN" -p="$DOCKER_PASSWORD" && \
docker push prest/prest:v1 && \
docker tag prest/prest:v1 prest/prest:$TRAVIS_TAG && \
docker push prest/prest:$TRAVIS_TAG && \
git checkout . && \
curl -sL https://git.io/goreleaser | bash
