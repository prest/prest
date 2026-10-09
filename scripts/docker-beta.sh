#!/usr/bin/env bash

# every commit that goes into the main branch we will generate a version in
# the beta tag in the github package (docker)
git checkout . && \
    docker buildx build --platform linux/amd64,linux/arm64 --push . -t ghcr.io/prest/prest:beta && \
    docker buildx build --platform linux/amd64,linux/arm64 --push . -f Dockerfile.noplugins -t ghcr.io/prest/prest:beta-noplugins
