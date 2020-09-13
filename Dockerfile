FROM golang:alpine as builder
COPY . /go/src/github.com/palevi67/prest
WORKDIR /go/src/github.com/palevi67/prest/cmd/prestd
ENV GO111MODULE=on
RUN apk add --no-cache git && \
        go mod tidy && go build

FROM alpine
COPY --from=builder /go/src/github.com/palevi67/prest/cmd/prestd/prestd /app/prestd
RUN apk add --no-cache curl
ADD ./cmd/prestd/prest.toml /app/prest.toml
ADD ./etc/entrypoint.sh /app/entrtpoint.sh
ENTRYPOINT [ "/app/entrtpoint.sh" ]
