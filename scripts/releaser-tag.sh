#!/usr/bin/env bash
if [ -n "$DOCKER_LOGIN" ] && [ -n "$DOCKER_PASSWORD" ]; then
	echo "Login to the docker..."
	echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_LOGIN" --password-stdin
fi

# Workaround for github actions when access to different repositories is needed.
# Github actions provides a GITHUB_TOKEN secret that can only access the current
# repository and you cannot configure it's value.
# Access to different repositories is needed by brew for example.

if [ -n "$GORELEASER_GITHUB_TOKEN" ] ; then
	export GITHUB_TOKEN=$GORELEASER_GITHUB_TOKEN
	echo "$GORELEASER_GITHUB_TOKEN" | docker login docker.pkg.github.com -u prest --password-stdin
	echo "$GORELEASER_GITHUB_TOKEN" | docker login ghcr.io -u prest --password-stdin
fi

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
    curl -sL https://git.io/goreleaser | bash
