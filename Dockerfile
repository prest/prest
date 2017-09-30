FROM golang:1.9-alpine as build-stage
WORKDIR /go/src/github.com/prest/prest
COPY  ./ /go/src/github.com/prest/prest
RUN go build

FROM alpine:3.6
COPY --from=build-stage /go/src/github.com/prest/prest/prest /
CMD "/prest"
