FROM golang:1.14-alpine as build-stage
WORKDIR /go/src/github.com/prest/prest
COPY  ./ /go/src/github.com/prest/prest
WORKDIR /go/src/github.com/prest/prest/cmd/prestd
RUN go get ./... && go build -ldflags "-s -w"

FROM alpine:3.7
COPY --from=build-stage /go/src/github.com/prest/prest/cmd/prestd/prestd /
COPY --from=build-stage /go/src/github.com/prest/prest/cmd/prestd/prest.toml /
ENTRYPOINT ["/prestd"]
