#!/usr/bin/env bash
export DOCKER_TAG=${GITHUB_REF#refs/tags/}

git checkout . && \
    docker build . -t ghcr.io/prest/prest:latest && \
    docker tag ghcr.io/prest/prest:latest prest/prest:$DOCKER_TAG && \
    docker tag ghcr.io/prest/prest:latest prest/prest:v1 && \
    docker tag ghcr.io/prest/prest:latest prest/prest:latest && \
    docker tag ghcr.io/prest/prest:latest ghcr.io/prest/prest:v1 && \
    docker tag ghcr.io/prest/prest:latest ghcr.io/prest/prest:$DOCKER_TAG && \
    docker push ghcr.io/prest/prest:latest && \
    docker push ghcr.io/prest/prest:v1 && \
    docker push ghcr.io/prest/prest:$DOCKER_TAG && \
    docker push prest/prest:latest && \
    docker push prest/prest:v1 && \
    docker push prest/prest:$DOCKER_TAG && \
    git checkout . && \
    rm cache/test && \
    curl -sL https://git.io/goreleaser | bash
