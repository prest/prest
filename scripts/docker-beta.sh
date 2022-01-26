#!/usr/bin/env bash

# every commit that goes into the main branch we will generate a version in
# the beta tag in the github package (docker)
git checkout . && \
    docker build . -t ghcr.io/prest/prest:beta && \
    docker push ghcr.io/prest/prest:beta
