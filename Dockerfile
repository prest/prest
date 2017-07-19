FROM golang:1.8-alpine

RUN mkdir -p /go/src/github.com/nuveo/prest
COPY  ./ /go/src/github.com/nuveo/prest
WORKDIR /go/src/github.com/nuveo/prest
RUN go install
CMD ["prest"]