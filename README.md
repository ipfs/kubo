![](https://ipfs.io/ipfs/QmVk7srrwahXLNmcDYvyUEJptyoxpndnRa57YJ11L4jV26/ipfs.go.png)

# IPFS implementation in Go
[![GoDoc](https://godoc.org/github.com/ipfs/go-ipfs?status.svg)](https://godoc.org/github.com/ipfs/go-ipfs) [![Build Status](https://travis-ci.org/ipfs/go-ipfs.svg?branch=master)](https://travis-ci.org/ipfs/go-ipfs)

IPFS is a global, versioned, peer-to-peer filesystem. It combines good ideas from
Git, BitTorrent, Kademlia, SFS, and the Web. It is like a single bittorrent swarm,
exchanging git objects. IPFS provides an interface as simple as the HTTP web, but
with permanence built in. You can also mount the world at /ipfs.

For more info see: https://github.com/ipfs/ipfs.

Please put all issues regarding IPFS _design_ in the
[ipfs repo issues](https://github.com/ipfs/ipfs/issues).
Please put all issues regarding Go IPFS _implementation_ in [this repo](https://github.com/ipfs/go-ipfs/issues).

## Table of Contents

- [Security Issues](#security-issues)
- [Install](#install)
  - [Install prebuilt packages](#install-prebuilt-packages)
  - [Build from Source](#build-from-source)
  - [Prerequisite: Install Go](#prerequisite-install-go)
  - [Download + Compile IPFS](#download--compile-ipfs)
  - [Development Dependencies](#development-dependencies)
- [Updating](#updating)
- [Usage](#usage)
- [Getting Started](#getting-started)
  - [Some things to try](#some-things-to-try)
  - [Docker usage](#docker-usage)
    - [Docker usage with VirtualBox/boot2docker (OSX and Windows)](#docker-usage-with-virtualboxboot2docker-osx-and-windows)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [Todo](#todo)
- [License](#license)

## Security Issues

The IPFS protocol and its implementations are still in heavy development. This means that there may be problems in our protocols, or there may be mistakes in our implementations. And -- though IPFS is not production-ready yet -- many people are already running nodes in their machines. So we take security vulnerabilities very seriously. If you discover a security issue, please bring it to our attention right away!

If you find a vulnerability that may affect live deployments -- for example, by exposing a remote execution exploit -- please send your report privately to security@ipfs.io. Please DO NOT file a public issue.

If the issue is a protocol weakness that cannot be immediately exploited or something not yet deployed, just discuss it openly.

## Install

The canonical download instructions for IPFS are over at: http://ipfs.io/docs/install/. It is **highly suggested** you follow those instructions if you are not interested in working on IPFS development.

### Install prebuilt packages

We host prebuilt binaries over at our [distributions page](https://ipfs.io/ipns/dist.ipfs.io#go-ipfs).

From there:
- Click the blue "Download go-ipfs" on the right side of the page.
- Open/extract the archive.
- Move `ipfs` to your path (`install.sh` can do it for you).

### Build from Source

#### Install Go

The build process for ipfs requires Go 1.5+, but we strongly recommend using 1.6.2. If you don't have it: [Download Go 1.6.2+](https://golang.org/dl/).


You'll need to add Go's bin directories to your `$PATH` environment variable e.g., by adding these lines to your `/etc/profile` (for a system-wide installation) or `$HOME/.profile`:

```
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:$GOPATH/bin
```

(If you run into trouble, see the [Go install instructions](https://golang.org/doc/install)).

#### Download + Compile IPFS

go-ipfs differs from the vanilla `go get` flow: it uses
[gx](https://github.com/whyrusleeping/gx)/[gx-go](https://github.com/whyrusleeping/gx-go)
for dependency management.

First download `go-ipfs` without installing:

```
$ go get -d github.com/ipfs/go-ipfs

$ cd $GOPATH/src/github.com/ipfs/go-ipfs
```

Then install `go-ipfs` and its dependencies, including `gx` and `gx-go`:

```
$ make install
```

#### Troubleshooting

* Separate [instructions are available for building on Windows](docs/windows.md).
* `git` is required in order for `go get` to fetch all dependencies.
* Package managers often contain out-of-date `golang` packages.
  Ensure that `go version` reports at least 1.5.2. See above for how to install go.
* If you are interested in development, please install the development
dependencies as well.
* *WARNING: Older versions of OSX FUSE (for Mac OS X) can cause kernel panics when mounting!*
  We strongly recommend you use the [latest version of OSX FUSE](http://osxfuse.github.io/).
  (See https://github.com/ipfs/go-ipfs/issues/177)
* For more details on setting up FUSE (so that you can mount the filesystem), see the docs folder.
* Shell command completion is available in `misc/completion/ipfs-completion.bash`. Read [docs/command-completion.md](docs/command-completion.md) to learn how to install it.
* See the [init examples](https://github.com/ipfs/examples/tree/master/examples/init) for how to connect IPFS to systemd or whatever init system your distro uses.

### Development Dependencies

If you make changes to the protocol buffers, you will need to install the [protoc compiler](https://github.com/google/protobuf).

## Updating
IPFS has an updating tool that can be accessed through `ipfs update`. The tool is
not installed alongside IPFS in order to keep that logic independent of the main
codebase. To install `ipfs update`, either [download it here](https://gobuilder.me/github.com/ipfs/ipfs-update)
or install it from source with `go get -u github.com/ipfs/ipfs-update`.

## Usage

```
  ipfs - Global p2p merkle-dag filesystem.

  ipfs [<flags>] <command> [<arg>] ...

SUBCOMMANDS
  BASIC COMMANDS
    init          Initialize ipfs local configuration
    add <path>    Add a file to ipfs
    cat <ref>     Show ipfs object data
    get <ref>     Download ipfs objects
    ls <ref>      List links from an object
    refs <ref>    List hashes of links from an object

  DATA STRUCTURE COMMANDS
    block         Interact with raw blocks in the datastore
    object        Interact with raw dag nodes
    files         Interact with objects as if they were a unix filesystem

  ADVANCED COMMANDS
    daemon        Start a long-running daemon process
    mount         Mount an ipfs read-only mountpoint
    resolve       Resolve any type of name
    name          Publish or resolve IPNS names
    dns           Resolve DNS links
    pin           Pin objects to local storage
    repo          Manipulate an IPFS repository

  NETWORK COMMANDS
    id            Show info about ipfs peers
    bootstrap     Add or remove bootstrap peers
    swarm         Manage connections to the p2p network
    dht           Query the DHT for values or peers
    ping          Measure the latency of a connection
    diag          Print diagnostics

  TOOL COMMANDS
    config        Manage configuration
    version       Show ipfs version information
    update        Download and apply go-ipfs updates
    commands      List all available commands

  Use 'ipfs <command> --help' to learn more about each command.

  ipfs uses a repository in the local file system. By default, the repo is located
  at ~/.ipfs. To change the repo location, set the $IPFS_PATH environment variable:

    export IPFS_PATH=/path/to/ipfsrepo
```

## Getting Started

See also: http://ipfs.io/docs/getting-started/

To start using IPFS, you must first initialize IPFS's config files on your
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

An IPFS docker image is hosted at [hub.docker.com/r/ipfs/go-ipfs](https://hub.docker.com/r/ipfs/go-ipfs/).
To make files visible inside the container you need to mount a host directory
with the `-v` option to docker. Choose a directory that you want to use to
import/export files from IPFS. You should also choose a directory to store
IPFS files that will persist when you restart the container.

    export ipfs_staging=</absolute/path/to/somewhere/>
    export ipfs_data=</absolute/path/to/somewhere_else/>

Make sure docker can access these folders:

    sudo chmod -R 777 /absolute/path/to/somewhere/
    sudo chmod -R 777 /absolute/path/to/somewhere_else/

Start a container running ipfs and expose ports 4001, 5001 and 8080:

    docker run -d --name ipfs_host -v $ipfs_staging:/export -v $ipfs_data:/data/ipfs -p 8080:8080 -p 4001:4001 -p 5001:5001 ipfs/go-ipfs:latest

Watch the ipfs log:

    docker logs -f ipfs_host

Wait for ipfs to start. ipfs is running when you see:

    Gateway (readonly) server
    listening on /ip4/0.0.0.0/tcp/8080

You can now stop watching the log.

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
relevant ports from the VM to your host for IPFS to act normally. This is
accomplished with the following command:

    boot2docker ssh -L 5001:localhost:5001 -L 4001:localhost:4001 -L 8080:localhost:8080 -fN

### Troubleshooting
If you have previously installed IPFS before and you are running into
problems getting a newer version to work, try deleting (or backing up somewhere
else) your IPFS config directory (~/.ipfs by default) and rerunning `ipfs init`.
This will reinitialize the config file to its defaults and clear out the local
datastore of any bad entries.

For any other problems, check the [issues list](https://github.com/ipfs/go-ipfs/issues)
and if you dont see your problem there, either come talk to us on irc (freenode #ipfs) or
file an issue of your own!

## Contributing

Please see [Contribute.md](contribute.md)!

## Todo

An IPFS alpha version has been released in February 2015. Things left to be done are all marked as [issues](https://github.com/ipfs/go-ipfs/issues).

## License

MIT
