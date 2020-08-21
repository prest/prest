#!/usr/bin/env sh
git clone https://github.com/prest/prest.github.io docbuild && \
    rm -rf docbuild/content && \
    cp -rf docs docbuild/content && \
    cd docbuild && \
    hugo $@
