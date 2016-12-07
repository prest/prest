FROM golang:1.7-alpine

RUN apk update && apk add curl git
RUN mkdir -p /go/src/github.com/nuveo/prest
COPY  ./ /go/src/github.com/nuveo/prest
WORKDIR /go/src/github.com/nuveo/prest
RUN go get -u github.com/kardianos/govendor
RUN govendor sync
RUN go install
CMD ["prest"]