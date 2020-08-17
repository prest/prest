FROM golang:alpine as builder
COPY . /go/src/github.com/prest/prest
WORKDIR /go/src/github.com/prest/prest/cmd/prestd
ENV GO111MODULE=on
RUN apk add --no-cache git && \
        go mod tidy && go build

FROM alpine
COPY --from=builder /go/src/github.com/prest/prest/cmd/prestd/prestd /app/prestd
ADD ./cmd/prestd/prest.toml /app/prest.toml
CMD ["/app/prestd"]
