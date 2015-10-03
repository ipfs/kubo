# Contribute

go-ipfs is MIT licensed open source software. We welcome contributions big and
small! Take a look at the [community contributing notes](https://github.com/ipfs/community/blob/master/contributing.md). Please make sure to check the [issues](https://github.com/ipfs/go-ipfs/issues). Search the closed ones
before reporting things, and help us with the open ones.

General Guidelines:

- see the [dev pseudo-roadmap](dev.md)
- please adhere to the protocol described in [the main ipfs repo](https://github.com/ipfs/ipfs), [paper](http://static.benet.ai/t/ipfs.pdf), and [specs](https://github.com/ipfs/specs) (WIP).
- please make branches + pull-request, even if working on the main repository
- ask questions or talk about things in [Issues](https://github.com/ipfs/go-ipfs/issues) or #ipfs on freenode.
- ensure you are able to contribute (no legal issues please-- we'll probably setup a CLA)
- run `go fmt` before pushing any code
- run `golint` and `go vet` too -- some things (like protobuf files) are expected to fail.
- if you'd like to work on ipfs part-time (20+ hrs/wk) or full-time (40+ hrs/wk), contact [@jbenet](https://github.com/jbenet)
- have fun!

A short intro to the Go development workflow:

- Ensure you have [Go installed on your system](https://golang.org/doc/install).
- Make sure that you have the environment variable `GOPATH` set somewhere, e.g. `$HOME/gopkg`
- Clone ipfs into the path `$GOPATH/src/github.com/ipfs/go-ipfs`
  - NOTE: This is true even if you have forked go-ipfs, dependencies in go are path based and must be in the right locations.
- You are now free to make changes to the codebase as you please.
- You can build the binary by running `go build ./cmd/ipfs` from the go-ipfs directory.
  - NOTE: when making changes remember to restart your daemon to ensure its running your new code.
    
