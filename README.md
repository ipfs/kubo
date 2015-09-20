# ipfs implementation in go.
[![GoDoc](https://godoc.org/github.com/ipfs/go-ipfs?status.svg)](https://godoc.org/github.com/ipfs/go-ipfs) [![Build Status](https://travis-ci.org/ipfs/go-ipfs.svg?branch=master)](https://travis-ci.org/ipfs/go-ipfs)

Ipfs is a global, versioned, peer-to-peer filesystem. It combines good ideas from
Git, BitTorrent, Kademlia, SFS, and the Web. It is like a single bittorrent swarm,
exchanging git objects. IPFS provides an interface as simple as the HTTP web, but
with permanence built in. You can also mount the world at /ipfs.

For more info see: https://github.com/ipfs/ipfs

Please put all issues regarding IPFS _design_ in the
[ipfs repo issues](https://github.com/ipfs/ipfs/issues).
Please put all issues regarding go IPFS _implementation_ in [this repo](https://github.com/ipfs/go-ipfs/issues).

## Install

The canonical download instructions for IPFS are over at: http://ipfs.io/docs/install

## Install prebuilt packages

We use [gobuilder.me](https://gobuilder.me), a great service that automatically builds a release on every commit.

You can see the latest builds for your platform at these links:

- [`release` - the last released version](https://gobuilder.me/github.com/ipfs/go-ipfs/cmd/ipfs?branch=release)  **<-- recommended**
- [`master` - development, stable](https://gobuilder.me/github.com/ipfs/go-ipfs/cmd/ipfs?branch=master)

From there:
- click "Download" on the build for your platform
- open/extract the archive
- move `ipfs` to your path (`install.sh` can do it for you)


## Build from Source

### Prerequisite: Install Go

First, you'll need go. If you don't have it: [Download Go 1.4+](https://golang.org/dl/).

You'll need to add Go's bin directories to your `$PATH` environment variable e.g., by adding these lines to your `/etc/profile` (for a system-wide installation) or `$HOME/.profile`:
```
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:$GOPATH/bin
```

(If you run into trouble, see the [Go install instructions](http://golang.org/doc/install))

### Download + Compile IPFS

Then simply:

```
go get -u github.com/ipfs/go-ipfs/cmd/ipfs
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
  (See https://github.com/ipfs/go-ipfs/issues/177)
* For more details on setting up FUSE (so that you can mount the filesystem), see the docs folder
* Shell command completion is available in `misc/completion/ipfs-completion.bash`. Read [docs/command-completion.md](docs/command-completion.md) to learn how to install it.
* See the [init examples](https://github.com/ipfs/examples/tree/master/examples/init) for how to connect IPFS to systemd or whatever init system your distro uses.


## Usage

```
USAGE:

    ipfs - global p2p merkle-dag filesystem

    ipfs [<flags>] <command> [<arg>] ...

    BASIC COMMANDS
    
        init          Initialize ipfs local configuration
        add <path>    Add an object to ipfs
        cat <ref>     Show ipfs object data
        get <ref>     Download ipfs objects
        ls <ref>      List links from an object
        refs <ref>    List hashes of links from an object
    
    DATA STRUCTURE COMMANDS
    
        block         Interact with raw blocks in the datastore
        object        Interact with raw dag nodes
        file          Interact with Unix filesystem objects
    
    ADVANCED COMMANDS
    
        daemon        Start a long-running daemon process
        mount         Mount an ipfs read-only mountpoint
        resolve       Resolve any type of name
        name          Publish or resolve IPNS names
        dns           Resolve DNS links
        pin           Pin objects to local storage
        repo gc       Garbage collect unpinned objects
    
    NETWORK COMMANDS
    
        id            Show info about ipfs peers
        bootstrap     Add or remove bootstrap peers
        swarm         Manage connections to the p2p network
        dht           Query the dht for values or peers
        ping          Measure the latency of a connection
        diag          Print diagnostics
    
    TOOL COMMANDS
    
        config        Manage configuration
        version       Show ipfs version information
        update        Download and apply go-ipfs updates
        commands      List all available commands
    
    Use 'ipfs <command> --help' to learn more about each command.


```

## Getting Started

See also: http://ipfs.io/docs/getting-started/

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


### Docker usage

An ipfs docker image is hosted at [hub.docker.com/r/jbenet/go-ipfs](http://hub.docker.com/r/jbenet/go-ipfs).
To make files visible inside the container you need to mount a host directory
with the `-v` option to docker. Choose a directory that you want to use to
import/export files from ipfs. You should also choose a directory to store
ipfs files that will persist when you restart the container.

    export ipfs_staging=</absolute/path/to/somewhere/>
    export ipfs_data=</absolute/path/to/somewhere_else/>

Start a container running ipfs and expose ports 4001, 5001 and 8080:

    docker run -d --name ipfs_host -v $ipfs_staging:/export -v $ipfs_data:/data/ipfs -p 8080:8080 -p 4001:4001 -p 5001:5001 jbenet/go-ipfs:latest

Watch the ipfs log:

    docker logs -f ipfs_host

Wait for ipfs to start. ipfs is running when you see:

    Gateway (readonly) server
    listening on /ip4/0.0.0.0/tcp/8080

(you can now stop watching the log)

Run ipfs commands:

    docker exec ipfs_host ipfs <args...>

For example: connect to peers

    docker exec ipfs_host ipfs swarm peers


Add files:

    cp -r <something> $ipfs_staging
    docker exec ipfs_host ipfs add -r /export/<something>

Stop the running container:

    docker stop ipfs_host

#### Docker usage with VirtualBox/boot2docker (OSX and Windows)

Since docker is running in the boot2docker VM, you need to forward
relevant ports from the VM to your host for ipfs act normally. This is
accomplished with the following command:

    boot2docker ssh -L 5001:localhost:5001 -L 4001:localhost:4001 -L 8080:localhost:8080 -fN


### Troubleshooting
If you have previously installed ipfs before and you are running into
problems getting a newer version to work, try deleting (or backing up somewhere
else) your ipfs config directory (~/.ipfs by default) and rerunning `ipfs init`.
This will reinitialize the config file to its defaults and clear out the local
datastore of any bad entries.

For any other problems, check the [issues list](http://github.com/ipfs/go-ipfs/issues)
and if you dont see your problem there, either come talk to us on irc (freenode #ipfs) or
file an issue of your own!


## Contributing

Please see [Contribute.md](contribute.md)!

## Todo

An IPFS alpha version has been released in February 2015. Things left to be done are all marked as [Issues](https://github.com/ipfs/go-ipfs/issues)

## Development Dependencies

If you make changes to the protocol buffers, you will need to install the [protoc compiler](https://code.google.com/p/protobuf/downloads/list).

## License

MIT
