FROM alpine:3.7
COPY prestd /prestd
COPY ./cmd/prestd/prest.toml /prest.toml
ENTRYPOINT ["/prestd"]
