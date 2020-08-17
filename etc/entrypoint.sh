#!/usr/bin/env sh

curl -sfL https://raw.githubusercontent.com/gofn/tcp-port-wait/master/tcp-port-wait.sh | sh -s -- postgres 5432
/app/prestd $@
