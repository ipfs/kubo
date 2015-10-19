all: install
commit = $(shell git rev-parse --short HEAD 2> /dev/null || echo unknown)
ldflags = "-X "github.com/ipfs/go-ipfs/repo/config".CurrentCommit=$(commit)"

build:
	go build -ldflags=$(ldflags)

install: build
	go install
