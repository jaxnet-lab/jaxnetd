include builder/Makefile-defaults.mk

all: dep build

dep:
	go mod vendor

#install:
#	cp configuration-manager $(DESTDIR)

build:
#	 go build -o app .

clean:
	go clean
	rm -fr vendor

.PHONY: all dep build clean install
