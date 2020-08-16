#!/usr/bin/env bash
docker build . -t prest/prest:v1 && \
docker push prest/prest:v1 && \
docker tag prest/prest:v1 prest/prest:$TRAVIS_TAG && \
docker push prest/prest:$TRAVIS_TAG && \
curl -sL https://git.io/goreleaser | bash
