FROM registry.hub.docker.com/library/golang:1.15 as builder
WORKDIR /workspace
COPY . .
RUN go mod download  \
    && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -ldflags "-s -w" -o prestd cmd/prestd/main.go \
    && apt-get update && apt-get install --no-install-recommends -yq netcat=1.10-41.1

# Use Distroless Docker Images
# tag "debug" because we need a shell (busybox)
FROM gcr.io/distroless/base:debug
COPY --from=builder /bin/nc /bin/nc
COPY --from=builder /workspace/prestd /bin/prestd
COPY --from=builder --chown=nonroot:nonroot /workspace/etc/prest.toml /app/prest.toml
COPY --from=builder --chown=nonroot:nonroot /workspace/etc/entrypoint.sh /app/entrypoint.sh
USER nonroot:nonroot
WORKDIR /app
ENTRYPOINT ["sh", "/app/entrypoint.sh"]
