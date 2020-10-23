include builder/Makefile-defaults.mk

all: build

build:
	go build -o shard.core
	go build -o jaxctl gitlab.com/jaxnet/core/shard.core/cmd/btcctl

clean:
	go clean
	rm -fr vendor

.PHONY: all dep build clean
build_docker_local:
	docker build --no-cache -t shard-core .

up_local: build_docker_local
	cd utils/docker && docker-compose up -d

push_to_dev:
	env GOOS=linux GOARCH=amd64 go build  -gcflags "all=-N -l"  .
	rsync -arv shard.core cl1sh1.jax:/opt/shard-core/
	# dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ../shard.core -C shard.local.yaml

