# kubo

> the oldest IPFS implementation, previously known as "go-ipfs"

![kubo, an IPFS node in Go](https://ipfs.io/ipfs/bafykbzacecaesuqmivkauix25v6i6xxxsvsrtxknhgb5zak3xxsg2nb4dhs2u/ipfs.go.png)

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square&cacheSeconds=3600)](https://protocol.ai)
[![GoDoc](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square&cacheSeconds=3600)](https://godoc.org/github.com/ipfs/kubo)
[![CircleCI](https://img.shields.io/circleci/build/github/ipfs/kubo?style=flat-square&cacheSeconds=3600)](https://circleci.com/gh/ipfs/kubo)

## What is Kubo?

Kubo (go-ipfs) the earliest and most widely used implementation of IPFS.

It includes:
- an IPFS daemon server
- extensive [command line tooling](https://docs.ipfs.tech/reference/kubo/cli/)
- an [HTTP Gateway](https://docs.ipfs.tech/reference/http/gateway/) (`/ipfs/`, `/ipns/`) for serving content to HTTP browsers
- an [HTTP RPC API](https://docs.ipfs.tech/reference/kubo/rpc/) (`/api/v0`) for controlling the daemon node

Note: [other implementations exist](https://docs.ipfs.tech/basics/ipfs-implementations/).

## What is IPFS?

IPFS is a global, versioned, peer-to-peer filesystem. It combines good ideas from previous systems such as Git, BitTorrent, Kademlia, SFS, and the Web. It is like a single BitTorrent swarm, exchanging git objects. IPFS provides an interface as simple as the HTTP web, but with permanence built-in. You can also mount the world at /ipfs.

For more info see: https://docs.ipfs.tech/concepts/what-is-ipfs/

Before opening an issue, consider using one of the following locations to ensure you are opening your thread in the right place:
  - kubo (previously named go-ipfs) _implementation_ bugs in [this repo](https://github.com/ipfs/kubo/issues).
  - Documentation issues in [ipfs/docs issues](https://github.com/ipfs/ipfs-docs/issues).
  - IPFS _design_ in [ipfs/specs issues](https://github.com/ipfs/specs/issues).
  - Exploration of new ideas in [ipfs/notes issues](https://github.com/ipfs/notes/issues).
  - Ask questions and meet the rest of the community at the [IPFS Forum](https://discuss.ipfs.tech).
  - Or [chat with us](https://docs.ipfs.tech/community/chat/).
 
[![YouTube Channel Subscribers](https://img.shields.io/youtube/channel/subscribers/UCdjsUXJ3QawK4O5L1kqqsew?label=Subscribe%20IPFS&style=social&cacheSeconds=3600)](https://www.youtube.com/channel/UCdjsUXJ3QawK4O5L1kqqsew) [![Follow @IPFS on Twitter](https://img.shields.io/twitter/follow/IPFS?style=social&cacheSeconds=3600)](https://twitter.com/IPFS)

## Next milestones

[Milestones on GitHub](https://github.com/ipfs/kubo/milestones)


## Table of Contents

- [What is Kubo?](#what-is-kubo)
- [What is IPFS?](#what-is-ipfs)
- [Next milestones](#next-milestones)
- [Table of Contents](#table-of-contents)
- [Security Issues](#security-issues)
- [Install](#install)
  - [System Requirements](#system-requirements)
  - [Docker](#docker)
  - [Official prebuilt binaries](#official-prebuilt-binaries)
    - [Updating](#updating)
      - [Using ipfs-update](#using-ipfs-update)
      - [Downloading builds using IPFS](#downloading-builds-using-ipfs)
  - [Unofficial Linux packages](#unofficial-linux-packages)
    - [ArchLinux](#arch-linux)
    - [Nix](#nix)
    - [Solus](#solus)
    - [openSUSE](#opensuse)
    - [Guix](#guix)
    - [Snap](#snap)
  - [Unofficial MacOS packages](#unofficial-macos-packages)
    - [MacPorts](#macports)
    - [Nix](#nix-1)
    - [Homebrew](#homebrew)  
  - [Unofficial Windows packages](#unofficial-windows-packages)
    - [Chocolatey](#chocolatey)
    - [Scoop](#scoop)
  - [Build from Source](#build-from-source)
    - [Install Go](#install-go)
    - [Download and Compile IPFS](#download-and-compile-ipfs)
      - [Cross Compiling](#cross-compiling)
      - [OpenSSL](#openssl)
    - [Troubleshooting](#troubleshooting)
- [Getting Started](#getting-started)
  - [Usage](#usage)
  - [Some things to try](#some-things-to-try)
  - [Troubleshooting](#troubleshooting-1)
- [Packages](#packages)
- [Development](#development)
  - [Map of Implemented Subsystems](#map-of-implemented-subsystems)
  - [CLI, HTTP-API, Architecture Diagram](#cli-http-api-architecture-diagram)
  - [Testing](#testing)
  - [Development Dependencies](#development-dependencies)
  - [Developer Notes](#developer-notes)
- [Maintainer Info](#maintainer-info)
- [Contributing](#contributing)
- [License](#license)

## Security Issues

Please follow [`SECURITY.md`](SECURITY.md).

## Install

The canonical download instructions for IPFS are over at: https://docs.ipfs.tech/install/. It is **highly recommended** you follow those instructions if you are not interested in working on IPFS development.

### System Requirements

IPFS can run on most Linux, macOS, and Windows systems. We recommend running it on a machine with at least 2 GB of RAM and 2 CPU cores (kubo is highly parallel). On systems with less memory, it may not be completely stable.

If your system is resource-constrained, we recommend:

1. Installing OpenSSL and rebuilding kubo manually with `make build GOTAGS=openssl`. See the [download and compile](#download-and-compile-ipfs) section for more information on compiling kubo.
2. Initializing your daemon with `ipfs init --profile=lowpower`

### Docker

Official images are published at https://hub.docker.com/r/ipfs/kubo/:

[![Docker Image Version (latest semver)](https://img.shields.io/docker/v/ipfs/kubo?color=blue&label=kubo%20docker%20image&logo=docker&sort=semver&style=flat-square&cacheSeconds=3600)](https://hub.docker.com/r/ipfs/kubo/)

More info on how to run Kubo (go-ipfs) inside Docker can be found [here](https://docs.ipfs.tech/how-to/run-ipfs-inside-docker/).

### Official prebuilt binaries

The official binaries are published at https://dist.ipfs.tech#kubo:

[![dist.ipfs.tech Downloads](https://img.shields.io/github/v/release/ipfs/kubo?label=dist.ipfs.tech&logo=ipfs&style=flat-square&cacheSeconds=3600)](https://dist.ipfs.tech#kubo)

From there:
- Click the blue "Download Kubo" on the right side of the page.
- Open/extract the archive.
- Move kubo (`ipfs`) to your path (`install.sh` can do it for you).

If you are unable to access [dist.ipfs.tech](https://dist.ipfs.tech#kubo), you can also download kubo (go-ipfs) from:
- this project's GitHub [releases](https://github.com/ipfs/kubo/releases/latest) page
- `/ipns/dist.ipfs.tech` at [dweb.link](https://dweb.link/ipns/dist.ipfs.tech#kubo) gateway

#### Updating

##### Using ipfs-update

IPFS has an updating tool that can be accessed through `ipfs update`. The tool is
not installed alongside IPFS in order to keep that logic independent of the main
codebase. To install `ipfs-update` tool, [download it here](https://dist.ipfs.tech/#ipfs-update).

##### Downloading builds using IPFS

List the available versions of Kubo (go-ipfs) implementation:

```console
$ ipfs cat /ipns/dist.ipfs.tech/kubo/versions
```

Then, to view available builds for a version from the previous command (`$VERSION`):

```console
$ ipfs ls /ipns/dist.ipfs.tech/kubo/$VERSION
```

To download a given build of a version:

```console
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_darwin-386.tar.gz    # darwin 32-bit build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_darwin-amd64.tar.gz  # darwin 64-bit build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_freebsd-amd64.tar.gz # freebsd 64-bit build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_linux-386.tar.gz     # linux 32-bit build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_linux-amd64.tar.gz   # linux 64-bit build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_linux-arm.tar.gz     # linux arm build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_windows-amd64.zip    # windows 64-bit build
```

### Unofficial Linux packages

- [Arch Linux](#arch-linux)
- [Nix](#nix-linux)
- [Solus](#solus)
- [openSUSE](#opensuse)

#### Arch Linux

[![kubo via Community Repo](https://img.shields.io/archlinux/v/community/x86_64/kubo?color=1793d1&label=kubo&logo=arch-linux&style=flat-square&cacheSeconds=3600)](https://wiki.archlinux.org/title/IPFS)

```bash
# pacman -S kubo
```

[![kubo-git via AUR](https://img.shields.io/static/v1?label=kubo-git&message=latest%40master&color=1793d1&logo=arch-linux&style=flat-square&cacheSeconds=3600)](https://aur.archlinux.org/packages/kubo/)

#### <a name="nix-linux">Nix</a>

With the purely functional package manager [Nix](https://nixos.org/nix/) you can install kubo (go-ipfs) like this:

```
$ nix-env -i ipfs
```

You can also install the Package by using its attribute name, which is also `ipfs`.

#### Solus

In solus, kubo (go-ipfs) is available in the main repository as
[go-ipfs](https://dev.getsol.us/source/go-ipfs/repository/master/).

```
$ sudo eopkg install go-ipfs
```

You can also install it through the Solus software center.

#### openSUSE

[Community Package for go-ipfs](https://software.opensuse.org/package/go-ipfs)

#### Guix

GNU's functional package manager, [Guix](https://www.gnu.org/software/guix/), also provides a go-ipfs package:

```
$ guix package -i go-ipfs
```

#### Snap

> ⚠️ **SNAP USE IS DISCOURAGED**
> 
> If you want something more sophisticated to escape the Snap confinement, we recommend using a different method to install Kubo so that it is not subject to snap confinement.


With snap, in any of the [supported Linux distributions](https://snapcraft.io/docs/core/install):

```
$ sudo snap install ipfs
```

The snap sets `IPFS_PATH` to `SNAP_USER_COMMON`, which is usually `~/snap/ipfs/common`. If you want to use `~/.ipfs` instead, you can bind-mount it to `~/snap/ipfs/common` like this:

```
$ sudo mount --bind ~/.ipfs ~/snap/ipfs/common
```

#### MacPorts

The package [ipfs](https://ports.macports.org/port/ipfs) currently points to kubo (go-ipfs) and is being maintained.

```
$ sudo port install ipfs
```

#### <a name="nix-macos">Nix</a>

In macOS you can use the purely functional package manager [Nix](https://nixos.org/nix/):

```
$ nix-env -i ipfs
```

You can also install the Package by using its attribute name, which is also `ipfs`.

#### Homebrew

A Homebrew formula [ipfs](https://formulae.brew.sh/formula/ipfs) is maintained too.

```
$ brew install --formula ipfs
```

### Unofficial Windows packages

- [Chocolatey](#chocolatey)
- [Scoop](#scoop)

#### Chocolatey

[![Chocolatey Version](https://img.shields.io/chocolatey/v/go-ipfs?color=00a4ef&label=go-ipfs&logo=windows&style=flat-square&cacheSeconds=3600)](https://chocolatey.org/packages/go-ipfs)

```Powershell
PS> choco install ipfs
```

#### Scoop

Scoop provides kubo as `go-ipfs` in its 'extras' bucket.

```Powershell
PS> scoop bucket add extras
PS> scoop install go-ipfs
```

### Unofficial macOS packages

- [MacPorts](#macports)
- [Nix](#nix-macos)
- [Homebrew](#homebrew)


### Build from Source

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/ipfs/kubo?label=Requires%20Go&logo=go&style=flat-square&cacheSeconds=3600)

kubo's build system requires Go and some standard POSIX build tools:

* GNU make
* Git
* GCC (or some other go compatible C Compiler) (optional)

To build without GCC, build with `CGO_ENABLED=0` (e.g., `make build CGO_ENABLED=0`).

#### Install Go

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/ipfs/kubo?label=Requires%20Go&logo=go&style=flat-square&cacheSeconds=3600)

If you need to update: [Download latest version of Go](https://golang.org/dl/).

You'll need to add Go's bin directories to your `$PATH` environment variable e.g., by adding these lines to your `/etc/profile` (for a system-wide installation) or `$HOME/.profile`:

```
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:$GOPATH/bin
```

(If you run into trouble, see the [Go install instructions](https://golang.org/doc/install)).

#### Download and Compile IPFS

```
$ git clone https://github.com/ipfs/kubo.git

$ cd kubo
$ make install
```

Alternatively, you can run `make build` to build the go-ipfs binary (storing it in `cmd/ipfs/ipfs`) without installing it.

**NOTE:** If you get an error along the lines of "fatal error: stdlib.h: No such file or directory", you're missing a C compiler. Either re-run `make` with `CGO_ENABLED=0` or install GCC.

##### Cross Compiling

Compiling for a different platform is as simple as running:

```
make build GOOS=myTargetOS GOARCH=myTargetArchitecture
```

##### OpenSSL

To build go-ipfs with OpenSSL support, append `GOTAGS=openssl` to your `make` invocation. Building with OpenSSL should significantly reduce the background CPU usage on nodes that frequently make or receive new connections.

Note: OpenSSL requires CGO support and, by default, CGO is disabled when cross-compiling. To cross-compile with OpenSSL support, you must:

1. Install a compiler toolchain for the target platform.
2. Set the `CGO_ENABLED=1` environment variable.

#### Troubleshooting

- Separate [instructions are available for building on Windows](docs/windows.md).
- `git` is required in order for `go get` to fetch all dependencies.
- Package managers often contain out-of-date `golang` packages.
  Ensure that `go version` reports at least 1.10. See above for how to install go.
- If you are interested in development, please install the development
dependencies as well.
- Shell command completions can be generated with one of the `ipfs commands completion` subcommands. Read [docs/command-completion.md](docs/command-completion.md) to learn more.
- See the [misc folder](https://github.com/ipfs/kubo/tree/master/misc) for how to connect IPFS to systemd or whatever init system your distro uses.

## Getting Started

### Usage

[![docs: Command-line quick start](https://img.shields.io/static/v1?label=docs&message=Command-line%20quick%20start&color=blue&style=flat-square&cacheSeconds=3600)](https://docs.ipfs.tech/how-to/command-line-quick-start/)
[![docs: Command-line reference](https://img.shields.io/static/v1?label=docs&message=Command-line%20reference&color=blue&style=flat-square&cacheSeconds=3600)](https://docs.ipfs.tech/reference/kubo/cli/)

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

### Troubleshooting

If you have previously installed IPFS before and you are running into problems getting a newer version to work, try deleting (or backing up somewhere else) your IPFS config directory (~/.ipfs by default) and rerunning `ipfs init`. This will reinitialize the config file to its defaults and clear out the local datastore of any bad entries.

Please direct general questions and help requests to our [forums](https://discuss.ipfs.tech).

If you believe you've found a bug, check the [issues list](https://github.com/ipfs/kubo/issues) and, if you don't see your problem there, either come talk to us on [Matrix chat](https://docs.ipfs.tech/community/chat/), or file an issue of your own!

## Packages

See [IPFS in GO](https://docs.ipfs.tech/reference/go/api/) documentation.

## Development

Some places to get you started on the codebase:

- Main file: [./cmd/ipfs/main.go](https://github.com/ipfs/kubo/blob/master/cmd/ipfs/main.go)
- CLI Commands: [./core/commands/](https://github.com/ipfs/kubo/tree/master/core/commands)
- Bitswap (the data trading engine): [go-bitswap](https://github.com/ipfs/go-bitswap)
- libp2p
  - libp2p: https://github.com/libp2p/go-libp2p
  - DHT: https://github.com/libp2p/go-libp2p-kad-dht
  - PubSub: https://github.com/libp2p/go-libp2p-pubsub
- [IPFS : The `Add` command demystified](https://github.com/ipfs/kubo/tree/master/docs/add-code-flow.md)

### Map of Implemented Subsystems
**WIP**: This is a high-level architecture diagram of the various sub-systems of this specific implementation. To be updated with how they interact. Anyone who has suggestions is welcome to comment [here](https://docs.google.com/drawings/d/1OVpBT2q-NtSJqlPX3buvjYhOnWfdzb85YEsM_njesME/edit) on how we can improve this!
<img src="https://docs.google.com/drawings/d/e/2PACX-1vS_n1FvSu6mdmSirkBrIIEib2gqhgtatD9awaP2_WdrGN4zTNeg620XQd9P95WT-IvognSxIIdCM5uE/pub?w=1446&amp;h=1036">

### CLI, HTTP-API, Architecture Diagram

![](./docs/cli-http-api-core-diagram.png)

> [Origin](https://github.com/ipfs/pm/pull/678#discussion_r210410924)

Description: Dotted means "likely going away". The "Legacy" parts are thin wrappers around some commands to translate between the new system and the old system. The grayed-out parts on the "daemon" diagram are there to show that the code is all the same, it's just that we turn some pieces on and some pieces off depending on whether we're running on the client or the server.

### Testing

```
make test
```

### Development Dependencies

If you make changes to the protocol buffers, you will need to install the [protoc compiler](https://github.com/google/protobuf).

### Developer Notes

Find more documentation for developers on [docs](./docs)

## Maintainer Info
* [Project Board for active and upcoming work](https://pl-strflt.notion.site/Kubo-GitHub-Project-Board-c68f9192e48e4e9eba185fa697bf0570)
* [Release Process](https://pl-strflt.notion.site/Kubo-Release-Process-5a5d066264704009a28a79cff93062c4)
* [Additional PL EngRes Kubo maintainer info](https://pl-strflt.notion.site/Kubo-go-ipfs-4a484aeeaa974dcf918027c300426c05)


## Contributing

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

We ❤️ all [our contributors](docs/AUTHORS); this project wouldn’t be what it is without you! If you want to help out, please see [CONTRIBUTING.md](CONTRIBUTING.md).

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

Please reach out to us in one [chat](https://docs.ipfs.tech/community/chat/) rooms.

## License

This project is dual-licensed under Apache 2.0 and MIT terms:

- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/ipfs/kubo/blob/master/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](https://github.com/ipfs/kubo/blob/master/LICENSE-MIT) or http://opensource.org/licenses/MIT)
