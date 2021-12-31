#!/usr/bin/env bash

# Clone doc-template repo only if 'docbuild' directory doesn't exist
[ -d docbuild ] || (git clone https://github.com/prest/doc-template docbuild)

# Update local template repo and build documentation
cd docbuild && \
    git fetch && \
    git pull --force && \
    rm -rf content && \
    cp -rf ../docs content/prestd && \
    hugo "$@"

