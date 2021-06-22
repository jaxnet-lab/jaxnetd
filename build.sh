#!/usr/bin/env bash

function git_commit() {
    if [ -d "./.git" ]; then
        commit=$(git rev-parse --short HEAD)
        diff_status=$(git diff-index HEAD)
        if [ "$diff_status" != "" ]; then
          commit=$commit-dirty.
        fi
        echo "$commit"
    else
        echo "n/a"
    fi
}

function git_tag() {
    if [ -d "./.git" ]; then
        git rev-parse --abbrev-ref HEAD
    else
        echo "n/a"
    fi
}

SERVICE="gitlab.com/jaxnet/jaxnetd"
## extract short hash of the current commit
COMMIT=$(git_commit)
## extract name of the current git branch or tag
TAG=$(git_tag)
DATE="$(date -I'seconds')"

PKG=$SERVICE/version
LD_FLAG="-X ${PKG}.commit=$COMMIT -X ${PKG}.tag=$TAG -X ${PKG}.date=${DATE}"

if [ "$1" != "" ]; then
  go build -o "${1}" -ldflags "$LD_FLAG" .
  exit 0
fi

go build -ldflags "$LD_FLAG" .
