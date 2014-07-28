# ipfs implementation in go.

See: https://github.com/jbenet/ipfs

Please put all issues regarding IPFS _design_ in the
[ipfs repo issues](https://github.com/jbenet/ipfs/issues).

Please put all issues regarding go IPFS _implementation_ in [this repo](https://github.com/jbenet/go-ipfs/issues).

## Install

[Install Go](http://golang.org/doc/install). Then:

```
go get github.com/jbenet/go-ipfs/cmd/ipfs
```

NOTE: `git` and mercurial (`hg`) are required in order for `go get` to fetch all dependencies.

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
