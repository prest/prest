FROM registry.hub.docker.com/library/golang:alpine as builder
COPY . /go/src/github.com/prest/prest
WORKDIR /go/src/github.com/prest/prest/cmd/prestd
ENV GO111MODULE=on
RUN apk add --no-cache git && \
        go mod tidy && go build

FROM registry.hub.docker.com/library/alpine
COPY --from=builder /go/src/github.com/prest/prest/cmd/prestd/prestd /app/prestd
RUN apk add --no-cache curl
ADD ./cmd/prestd/prest.toml /app/prest.toml
ADD ./etc/entrypoint.sh /app/entrtpoint.sh
ENTRYPOINT [ "/app/entrtpoint.sh" ]
