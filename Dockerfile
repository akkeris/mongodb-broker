FROM golang:1.13-alpine as builder

RUN apk update && \
    apk add openssl ca-certificates git make build-base

WORKDIR /go/src/github.com/akkeris/mongodb-broker

COPY . .

RUN make

FROM alpine:latest

WORKDIR /app

COPY --from=builder /go/src/github.com/akkeris/mongodb-broker/mongodb-broker mongodb-broker
COPY --from=builder /go/src/github.com/akkeris/mongodb-broker/start.sh start.sh
COPY --from=builder /go/src/github.com/akkeris/mongodb-broker/start-background.sh start-background.sh

CMD ./start.sh
