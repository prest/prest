#!/bin/sh

if [ "$DATABASE_URL" ];
then
    ## Parse DATABASE URL
    PROTO="$(echo $DATABASE_URL | grep :// | sed -e's,^\(.*://\).*,\1,g')"
    # remove the protocol
    URL="$(echo ${DATABASE_URL/$PROTO/})"
    # extract the user (if any)
    USER="$(echo $URL | grep @ | cut -d@ -f1)"
    # extract the host and port
    HOSTPORT="$(echo ${URL/$USER@/} | cut -d/ -f1)"
    # by request host without port
    PREST_PG_HOST="$(echo $HOSTPORT | sed -e 's,:.*,,g')"
    # by request - try to extract the port
    PREST_PG_PORT="$(echo $HOSTPORT | sed -e 's,^.*:,:,g' -e 's,.*:\([0-9]*\).*,\1,g' -e 's,[^0-9],,g')"
    echo "host: $PREST_PG_HOST | port: $PREST_PG_PORT"
fi

echo "[prestd] Waiting for port $PREST_PG_HOST:$PREST_PG_PORT to become available..."
while "! nc -z $PREST_PG_HOST $PREST_PG_PORT" 2>/dev/null
do
    ((elapsed=elapsed+1))
    if [ "$elapsed" -gt 90 ]
    then
        echo "[prestd] TIMED OUT!"
        exit 1
    fi
    sleep 1;
done

# prestd/plugin build
sh ./plugin/go-build.sh

sleep 5;
echo "[prestd] Ready hosting $PREST_PG_HOST to port $PREST_PG_PORT !"
/bin/prestd $@
