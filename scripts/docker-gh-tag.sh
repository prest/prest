#!/usr/bin/env bash
export DOCKER_TAG=${GITHUB_REF#refs/tags/}

git checkout . && \
    # Build the default image
    docker build . -t ghcr.io/prest/prest:latest && \
    docker tag ghcr.io/prest/prest:latest ghcr.io/prest/prest:v1 && \
    docker tag ghcr.io/prest/prest:latest ghcr.io/prest/prest:"$DOCKER_TAG" && \
    docker push ghcr.io/prest/prest:latest && \
    docker push ghcr.io/prest/prest:v1 && \
    docker push ghcr.io/prest/prest:"$DOCKER_TAG" && \
    # Build the noplugins image
    docker build . -f Dockerfile.noplugins -t ghcr.io/prest/prest:"latest-noplugins" && \
    docker tag ghcr.io/prest/prest:latest ghcr.io/prest/prest:"v1-noplugins" && \
    docker tag ghcr.io/prest/prest:latest ghcr.io/prest/prest:"$DOCKER_TAG-noplugins" && \
    docker push ghcr.io/prest/prest:"latest-noplugins" && \
    docker push ghcr.io/prest/prest:"v1-noplugins" && \
    docker push ghcr.io/prest/prest:"$DOCKER_TAG-noplugins"
