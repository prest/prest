FROM registry.hub.docker.com/library/golang:1.17 as builder
WORKDIR /workspace
COPY . .
ENV GOOS linux
ENV CGO_ENABLED 1
RUN go mod vendor && \
    go build -ldflags "-s -w" -o prestd cmd/prestd/main.go && \
    apt-get update && apt-get install --no-install-recommends -yq netcat

# Use golang image
# needs go to compile the plugin system
FROM registry.hub.docker.com/library/golang:1.17
ENV CGO_ENABLED 1
COPY --from=builder /bin/nc /bin/nc
COPY --from=builder /workspace/prestd /bin/prestd
COPY --from=builder /workspace/etc/prest.toml /app/prest.toml
COPY --from=builder /workspace/etc/entrypoint.sh /app/entrypoint.sh
COPY --from=builder /workspace/lib /app/lib
COPY --from=builder /workspace/etc/plugin /app/plugin
WORKDIR /app
ENTRYPOINT ["sh", "/app/entrypoint.sh"]
