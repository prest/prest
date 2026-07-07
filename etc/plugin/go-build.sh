#!/bin/bash
set -e
BUILD_FLAGS=(-trimpath -ldflags="-s -w")
echo "[prestd] Go build simple plugins in only file!"
for fullpath in ./lib/src/*.go; do
	if [ -f "$fullpath" ]; then
		filename=${fullpath##*/}
		filename_outext=${filename%%.*}
		echo "go build: $filename_outext plugin..." && \
		go build -mod=vendor "${BUILD_FLAGS[@]}" -o "./lib/${filename_outext}.so" -buildmode=plugin "./lib/src/${filename_outext}.go";
	fi
done

echo "[prestd] Go build complex plugins in folder (with main.go file)!"
for paths in ./lib/src/*/; do
	if [ -d "$paths" ]; then
		file_main="${paths}/main.go"
		if [ -f "${file_main}" ]; then
			filename_outext=${paths#./lib/src/}
			filename_outext=${filename_outext%/}
			echo "go build: ${filename_outext} plugin..." && \
			go build -mod=vendor "${BUILD_FLAGS[@]}" -o "./lib/${filename_outext}.so" -buildmode=plugin "${file_main}";
		fi
	fi
done