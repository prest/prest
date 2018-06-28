
FROM alpine:3.7
COPY prest /prest
ENTRYPOINT ["/prest"]
