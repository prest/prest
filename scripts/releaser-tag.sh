#!/usr/bin/env bash
export DOCKER_TAG=${GITHUB_REF#refs/tags/}

git checkout . && \
    rm cache/test && \
    curl -sL https://git.io/goreleaser | bash
