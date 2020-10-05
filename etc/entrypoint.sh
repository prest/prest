echo Waiting for port $PREST_PG_HOST:$PREST_PG_PORT to become available...
while ! nc -z $PREST_PG_HOST $PREST_PG_PORT 2>/dev/null
do
    let elapsed=elapsed+1
    if [ "$elapsed" -gt 90 ]
    then
        echo "TIMED OUT !"
        exit 1
    fi
    sleep 1;
done

sleep 5;
echo "Ready hosting $PREST_PG_HOST to port $PREST_PG_PORT !"
/app/prestd $@
