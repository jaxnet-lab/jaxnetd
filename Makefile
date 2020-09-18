include builder/Makefile-defaults.mk

all: dep build

dep:
	go mod vendor

#install:
#	cp configuration-manager $(DESTDIR)

build:
	 go build -o shardcore .

clean:
	go clean
	rm -fr vendor

.PHONY: all dep build clean install

build_docker_local:
	docker build --no-cache -t shard-core .

up_local: build_docker_local
	cd utils/docker && docker-compose up -d
