export GOMODULES11 = on
export GOPRIVATE = gitlab.com/jaxnet/core/*
DESTDIR ?= /output/
PACKAGE ?= $(shell basename $(PWD))
VERSION ?= $(shell git tag -l 'versions/*' | tail -n1 | cut -d/ -f2)
CI_COMMIT_SHORT_SHA  ?= $(shell git rev-parse HEAD)
