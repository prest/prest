#!/usr/bin/env sh
docker build . -t prest/prest:latest && \
docker push prest/prest:latest
