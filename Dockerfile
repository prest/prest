FROM golang:1.15 as builder

WORKDIR /workspace
COPY . .
WORKDIR /workspace/cmd/prestd
RUN go mod download

# Build
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o prestd main.go

RUN apt-get update && apt-get install -yq netcat

WORKDIR /app


# use debug because we need a shell (busybox)
FROM gcr.io/distroless/base:debug 
COPY --from=builder /bin/nc /bin/nc
COPY --from=builder --chown=nonroot:nonroot  /app /app
COPY --from=builder --chown=nonroot:nonroot  /workspace/cmd/prestd/prestd /app/prestd
COPY --from=builder --chown=nonroot:nonroot /workspace/cmd/prestd/prest.toml /app/prest.toml
COPY --from=builder --chown=nonroot:nonroot /workspace/etc/entrypoint.sh /app/entrypoint.sh
USER nonroot:nonroot
WORKDIR /app
ENTRYPOINT [ "sh", "/app/entrypoint.sh"]
