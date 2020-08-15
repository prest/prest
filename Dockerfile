FROM alpine:3.7
COPY prestd /app/prestd
COPY ./cmd/prestd/prest.toml /app/prest.toml
CMD ["/app/prestd"]
