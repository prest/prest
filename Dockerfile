FROM golang:1.8-alpine

RUN mkdir -p /go/src/github.com/prest/prest
COPY  ./ /go/src/github.com/prest/prest
WORKDIR /go/src/github.com/prest/prest
RUN go install
CMD ["prest"]
