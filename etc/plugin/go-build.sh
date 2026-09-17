#!/bin/bash
# Build pREST Go plugins with the same module/build identity as prestd when
# possible: vendor + trimpath + -ldflags "-s -w". See issue #948.
set -euo pipefail

PLUGIN_FLAGS=(-buildmode=plugin -trimpath -ldflags "-s -w")
if [ -d ./vendor ]; then
	PLUGIN_FLAGS=(-mod=vendor "${PLUGIN_FLAGS[@]}")
else
	echo "[prestd] warning: ./vendor missing; plugins may fail to load (build identity mismatch). Prefer 'go mod vendor' then rebuild."
fi

echo "[prestd] Go build simple plugins in only file!"
for fullpath in ./lib/src/*.go; do
	if [ -f "$fullpath" ]; then
		filename=${fullpath##*/}
		filename_outext=${filename%%.*}
		echo "go build: $filename_outext plugin..." && \
			go build "${PLUGIN_FLAGS[@]}" -o "./lib/${filename_outext}.so" "./lib/src/${filename_outext}.go"
	fi
done

echo "[prestd] Go build middleware plugins!"
mkdir -p ./lib/middlewares
for fullpath in ./lib/src/middlewares/*.go; do
	if [ -f "$fullpath" ]; then
		filename=${fullpath##*/}
		filename_outext=${filename%%.*}
		echo "go build: middlewares/$filename_outext plugin..." && \
			go build "${PLUGIN_FLAGS[@]}" -o "./lib/middlewares/${filename_outext}.so" "./lib/src/middlewares/${filename_outext}.go"
	fi
done

echo "[prestd] Go build complex plugins in folder (with main.go file)!"
for paths in $(ls -d ./lib/src/* 2>/dev/null || true); do
	if [ -d "$paths" ]; then
		file_main="${paths}/main.go"
		if [ -f "${file_main}" ]; then
			filename_outext=${paths/\.\/lib\/src\//""}
			echo "go build: ${filename_outext} plugin..." && \
				go build "${PLUGIN_FLAGS[@]}" -o "./lib/${filename_outext}.so" "${file_main}"
		fi
	fi
done
