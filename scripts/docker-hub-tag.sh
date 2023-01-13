#!/usr/bin/env bash
export DOCKER_TAG=${GITHUB_REF#refs/tags/}

git checkout . && \
    docker build . -t prest/prest:latest && \
    docker tag prest/prest:latest prest/prest:$DOCKER_TAG && \
    docker tag prest/prest:latest prest/prest:v1 && \
    docker push prest/prest:latest && \
    docker push prest/prest:v1 && \
    docker push prest/prest:$DOCKER_TAG && \
    git checkout . && \
    rm cache/test
