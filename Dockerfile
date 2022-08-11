# Compile stage
FROM golang:alpine AS build-env
RUN apk add --no-cache git bash

#ENV GOPROXY=direct
ENV GO111MODULE=on
ENV GOPRIVATE=gitlab.com


WORKDIR /shard-core
ADD . .
RUN ./build.sh /jaxnetd && CGO_ENABLED=0 go build -o /jaxctl gitlab.com/jaxnet/jaxnetd/cmd/jaxctl

# Final stage
FROM alpine:3.7

# Allow delve to run on Alpine based containers.
RUN apk add --no-cache ca-certificates bash

WORKDIR /

COPY --from=build-env /jaxnetd /
COPY --from=build-env /jaxctl /
COPY /jaxnetd.testnet.toml /
COPY /jaxnetd.mainnet.toml /

# default p2p port
EXPOSE 18444
# default rpc port
EXPOSE 18333
# default prometheus monitoring port
EXPOSE 18222

# Run app
CMD ./jaxnetd -C jaxnetd.testnet.toml
