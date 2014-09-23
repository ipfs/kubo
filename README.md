# ipfs implementation in go. [![GoDoc](https://godoc.org/github.com/jbenet/go-ipfs?status.svg)](https://godoc.org/github.com/jbenet/go-ipfs)

See: https://github.com/jbenet/ipfs

Please put all issues regarding IPFS _design_ in the
[ipfs repo issues](https://github.com/jbenet/ipfs/issues).
Please put all issues regarding go IPFS _implementation_ in [this repo](https://github.com/jbenet/go-ipfs/issues).

## Install

[Install Go](http://golang.org/doc/install). Then:

```
go get github.com/jbenet/go-ipfs/cmd/ipfs
cd $GOPATH/src/github.com/jbenet/go-ipfs/cmd/ipfs
go install
```

NOTE: `git` and mercurial (`hg`) are required in order for `go get` to fetch all dependencies.

If you are interested in development, please install the development dependencies as well.

## Usage

```
ipfs - global versioned p2p merkledag file system

Basic commands:

    add <path>    Add an object to ipfs.
    cat <ref>     Show ipfs object data.
    ls <ref>      List links from an object.
    refs <ref>    List link hashes from an object.

Tool commands:

    config        Manage configuration.
    version       Show ipfs version information.
    commands      List all available commands.

Advanced Commands:

    mount         Mount an ipfs read-only mountpoint.

Use "ipfs help <command>" for more information about a command.
```

## Contributing

go-ipfs is MIT licensed open source software. We welcome contributions big and small! Please make sure to check the [issues](https://github.com/jbenet/go-ipfs/issues). Search the closed ones before reporting things, and help us with the open ones.

Guidelines:

- see the [dev pseudo-roadmap](dev.md)
- please adhere to the protocol described in [the main ipfs repo](https://github.com/jbenet/ipfs) and [paper](http://static.benet.ai/t/ipfs.pdf).
- please make branches + pull-request, even if working on the main repository
- ask questions or talk about things in [Issues](https://github.com/jbenet/go-ipfs/issues) or #ipfs on freenode.
- ensure you are able to contribute (no legal issues please-- we'll probably setup a CLA)
- run `go fmt` before pushing any code
- run `golint` and `go vet` too -- some things (like protobuf files) are expected to fail.
- if you'd like to work on ipfs part-time (20+ hrs/wk) or full-time (40+ hrs/wk), contact [@jbenet](https://github.com/jbenet)
- have fun!

## Todo

IPFS is nearing an alpha release. Things left to be done are all marked as [Issues](https://github.com/jbenet/go-ipfs/issues)

## Development Dependencies

If you make changes to the protocol buffers, you will need to install the [protoc compiler](https://code.google.com/p/protobuf/downloads/list).

## License

MIT
