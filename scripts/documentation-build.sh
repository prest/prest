#!/usr/bin/env bash
rm -rf docbuild && \
git clone https://github.com/prest/doc-template docbuild && \
rm -rf docbuild/content && \
cp -rf docs docbuild/content && \
cd docbuild && \
hugo $@
