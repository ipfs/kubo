all: install
commit = $(shell git rev-parse --short HEAD 2> /dev/null || echo unknown)
ldflags = "-X "github.com/ipfs/go-ipfs/repo/config".CurrentCommit=$(commit)"

gx:
	go get -u github.com/whyrusleeping/gx
	go get -u github.com/whyrusleeping/gx-go

deps: gx
	gx --verbose install --global

build: deps
	go build -ldflags=$(ldflags)

install: build
	go install
