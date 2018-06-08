FROM golang:1.10-alpine as build-stage
WORKDIR /go/src/github.com/prest/prest
COPY  ./ /go/src/github.com/prest/prest
RUN go build -ldflags "-s -w"

FROM alpine:3.7
COPY --from=build-stage /go/src/github.com/prest/prest/prest /
COPY --from=build-stage /go/src/github.com/prest/prest/prest.toml /
CMD "/prest"
