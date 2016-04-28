COMMIT := $(shell git rev-parse --short HEAD)
ldflags = "-X "github.com/ipfs/go-ipfs/repo/config".CurrentCommit=$(COMMIT)"

all: install

install:
	go install -ldflags=$(ldflags)

build:
	go build -i -ldflags=$(ldflags)

nofuse:
	go install -tags nofuse -ldflags=$(ldflags)

clean:
	go clean -ldflags=$(ldflags)

uninstall:
	go clean -i -ldflags=$(ldflags)
