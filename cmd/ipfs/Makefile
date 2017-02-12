COMMIT := $(shell git rev-parse --short HEAD)
GOFLAGS += -ldflags="-X "github.com/ipfs/go-ipfs/repo/config".CurrentCommit=$(COMMIT)"

all: install

install:
	go install $(GOFLAGS)

build:
	go build -i $(GOFLAGS)

clean:
	go clean $(GOFLAGS)

uninstall:
	go clean -i $(GOFLAGS)
