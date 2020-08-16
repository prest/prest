#!/usr/bin/env bash
docker build . -t prest/prest:latest && \
docker push prest/prest:latest
