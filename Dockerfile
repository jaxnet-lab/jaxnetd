FROM golang:1.14-alpine

ARG CI_JOB_TOKEN

RUN apk add --no-cache git musl-dev gcc g++ make ca-certificates

RUN echo -e "machine gitlab.com\nlogin gitlab-ci-token\npassword ${CI_JOB_TOKEN}" > ~/.netrc

RUN go get -u github.com/goware/modvendor
WORKDIR /go/src/gitlab.com/jaxnet/core/shard.core

COPY . .

RUN GOARCH=amd64 GOOS=linux GO111MODULE=on make

FROM alpine:latest

RUN apk --no-cache add openssl ca-certificates

WORKDIR /root/

COPY --from=0 /go/src/gitlab.com/jaxnet/core/shard.core .

RUN chmod +x ./shardcore

EXPOSE 8444
EXPOSE 8334

CMD ./shardcore