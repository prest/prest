#!/bin/sh
echo "[prestd] Build plugins write in Go!"
for fullpath in ./lib/src/*.go; do
    if [ -f "$fullpath" ]; then
		filename=${fullpath##*/}
		filename_outext=${filename%%.*}
		echo "go build: $filename_outext plugin..." && \
			go build -o ./lib/${filename_outext}.so -buildmode=plugin ./lib/src/${filename_outext}.go;
    fi
done
