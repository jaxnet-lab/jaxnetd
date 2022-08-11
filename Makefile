include builder/Makefile-defaults.mk

all: build

build_all:
	CGO_ENABLED=0 go build -o ./bin/ ./...

build:
	CGO_ENABLED=0 go build -o jaxnetd
	CGO_ENABLED=0 go build -o jaxctl gitlab.com/jaxnet/jaxnetd/cmd/jaxctl

clean:
	go clean
	rm -fr vendor

.PHONY: all dep build clean
build_docker_local:
	docker build --no-cache -t jaxnetd .

up_local: build_docker_local
	cd utils/docker && docker-compose up -d

debug_build:
	#env GOOS=linux GOARCH=amd64
	go build  -gcflags "all=-N -l" -o ./jaxnetd .
	dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./jaxnetd -C shard.local.toml

