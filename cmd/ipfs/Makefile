all: install
commit = `git rev-parse --short HEAD`
ldflags = "-X "github.com/ipfs/go-ipfs/repo/config".CurrentCommit=$(commit)"

build:
	go build -ldflags=$(ldflags)

install: build
	go install
