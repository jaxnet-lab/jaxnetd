# Compile stage
FROM golang:alpine AS build-env
RUN apk add --no-cache git bash

#ENV GOPROXY=direct
ENV GO111MODULE=on
ENV GOPRIVATE=gitlab.com


WORKDIR /shard-core
ADD . .
RUN go build -o /shard.core && go build -o /jaxctl gitlab.com/jaxnet/core/shard.core/cmd/btcctl

# Final stage
FROM alpine:3.7

# Allow delve to run on Alpine based containers.
RUN apk add --no-cache ca-certificates bash

WORKDIR /

COPY --from=build-env /shard.core /
COPY --from=build-env /jaxctl /

# Run app
CMD ./shard.core
