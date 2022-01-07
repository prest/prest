#!/usr/bin/env bash

# Clone doc-template repo only if 'docbuild' directory doesn't exist
[ -d docbuild ] || (git clone --recurse-submodules https://github.com/prest/doc-template docbuild)

# Update local template repo and build documentation
cd docbuild && \
	git submodule update --rebase --remote && \
    cp -rf ../docs content/prestd && \
	cp -rf ../docs/assets static/prestd-assets && \
    hugo "$@"

