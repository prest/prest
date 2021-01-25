/*
pREST - Serve a RESTful API from any PostgreSQL database

Install
    go install github.com/prest/prest

Run

    Params:

    - PREST\_HTTP_PORT (default 3000)
    - PREST\_PG_HOST (default 127.0.0.1)
    - PREST\_PG_USER
    - PREST\_PG_PASS
    - PREST\_PG_DATABASE
    - PREST\_PG_PORT
    - PREST\_JWT_KEY

    ```
    PREST_PG_USER=postgres PREST_PG_DATABASE=prest PREST_PG_PORT=5432 PREST_HTTP_PORT=3010 prest # Binary installed
    ```

pREST also can be used as web framework, to see more details please visit https://docs.prestd.com/
*/
package main
