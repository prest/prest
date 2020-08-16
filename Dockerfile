FROM golang:alpine as builder

COPY . /go/src/github.com/prest/prest
WORKDIR /go/src/github.com/prest/prest
ENV GO111MODULE=on
RUN apk add --no-cache git && \
        go build ./cmd/prestd/

FROM alpine

COPY --from=builder /go/src/github.com/prest/prest/prestd /app/prestd
ADD ./cmd/prestd/prest.toml /app/prest.toml
CMD ["/app/prestd"]
