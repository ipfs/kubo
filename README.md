# ipfs implementation in go.
[![GoDoc](https://godoc.org/github.com/jbenet/go-ipfs?status.svg)](https://godoc.org/github.com/jbenet/go-ipfs) [![Build Status](https://travis-ci.org/jbenet/go-ipfs.svg?branch=master)](https://travis-ci.org/jbenet/go-ipfs)

Ipfs is a global, versioned, peer-to-peer filesystem. It combines good ideas from
Git, BitTorrent, Kademlia, SFS, and the Web. It is like a single bittorrent swarm,
exchanging git objects. IPFS provides an interface as simple as the HTTP web, but
with permanence built in. You can also mount the world at /ipfs.

For more info see: https://github.com/jbenet/ipfs

Please put all issues regarding IPFS _design_ in the
[ipfs repo issues](https://github.com/jbenet/ipfs/issues).
Please put all issues regarding go IPFS _implementation_ in [this repo](https://github.com/jbenet/go-ipfs/issues).

## Install

[Install Go 1.3+](http://golang.org/doc/install). Then simply:

```
go get -u github.com/jbenet/go-ipfs/cmd/ipfs
```

NOTES:

* `git` is required in order for `go get` to fetch
all dependencies.
* Package managers often contain out-of-date `golang` packages.
  Compilation from source is recommended.
* If you are interested in development, please install the development
dependencies as well.
* *WARNING: older versions of OSX FUSE (for Mac OS X) can cause kernel panics when mounting!*
  We strongly recommend you use the [latest version of OSX FUSE](http://osxfuse.github.io/).
  (See https://github.com/jbenet/go-ipfs/issues/177)


## Usage

```
    ipfs - global p2p merkle-dag filesystem

    ipfs [<flags>] <command> [<arg>] ...

    Basic commands:
    
        init          Initialize ipfs local configuration
        add <path>    Add an object to ipfs
        cat <ref>     Show ipfs object data
        ls <ref>      List links from an object
    
    Tool commands:
    
        config        Manage configuration
        update        Download and apply go-ipfs updates
        version       Show ipfs version information
        commands      List all available commands
        id            Show info about ipfs peers
    
    Advanced Commands:
    
        daemon        Start a long-running daemon process
        mount         Mount an ipfs read-only mountpoint
        serve         Serve an interface to ipfs
        diag          Print diagnostics
    
    Plumbing commands:
    
        block         Interact with raw blocks in the datastore
        object        Interact with raw dag nodes
    
    Use 'ipfs <command> --help' to learn more about each command.
```

## Getting Started
To start using ipfs, you must first initialize ipfs's config files on your
system, this is done with `ipfs init`. See `ipfs init --help` for information on
the optional arguments it takes. After initialization is complete, you can use
`ipfs mount`, `ipfs add` and any of the other commands to explore!

### Some things to try

Basic proof of 'ipfs working' locally:

	echo "hello world" > hello
	ipfs add hello
	# This should output a hash string that looks something like:
	# QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o
	ipfs cat <that hash>


### Troubleshooting
If you have previously installed ipfs before and you are running into
problems getting a newer version to work, try deleting (or backing up somewhere
else) your ipfs config directory (~/.go-ipfs by default) and rerunning `ipfs init`.
This will reinitialize the config file to its defaults and clear out the local
datastore of any bad entries.

For any other problems, check the [issues list](http://github.com/jbenet/go-ipfs/issues)
and if you dont see your problem there, either come talk to us on irc (freenode #ipfs) or
file an issue of your own!


## Contributing

go-ipfs is MIT licensed open source software. We welcome contributions big and
small! Please make sure to check the
[issues](https://github.com/jbenet/go-ipfs/issues). Search the closed ones
before reporting things, and help us with the open ones.

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
