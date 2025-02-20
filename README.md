<h1 align="center">
  <br>
  <a href="https://docs.ipfs.tech/how-to/command-line-quick-start/"><img src="https://user-images.githubusercontent.com/157609/250148884-d6d12db8-fdcf-4be3-8546-2550b69845d8.png" alt="Kubo logo" title="Kubo logo" width="200"></a>
  <br>
  Kubo: IPFS Implementation in GO
  <br>
</h1>

<p align="center" style="font-size: 1.2rem;">The first implementation of IPFS.</p>

<p align="center">
  <a href="https://ipfs.tech"><img src="https://img.shields.io/badge/project-IPFS-blue.svg?style=flat-square" alt="Official Part of IPFS Project"></a>
  <a href="https://discuss.ipfs.tech"><img alt="Discourse Forum" src="https://img.shields.io/discourse/posts?server=https%3A%2F%2Fdiscuss.ipfs.tech"></a>
  <a href="https://matrix.to/#/#ipfs-space:ipfs.io"><img alt="Matrix" src="https://img.shields.io/matrix/ipfs-space%3Aipfs.io?server_fqdn=matrix.org"></a>
  <a href="https://github.com/ipfs/kubo/actions"><img src="https://img.shields.io/github/actions/workflow/status/ipfs/kubo/build.yml?branch=master" alt="ci"></a>
  <a href="https://github.com/ipfs/kubo/releases"><img alt="GitHub release" src="https://img.shields.io/github/v/release/ipfs/kubo?filter=!*rc*"></a>
  <a href="https://godoc.org/github.com/ipfs/kubo"><img src="https://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square" alt="godoc reference"></a>  
</p>

<hr />

## What is Kubo?

Kubo was the first IPFS implementation and is the most widely used one today. Implementing the *Interplanetary Filesystem* - the Web3 standard for content-addressing, interoperable with HTTP. Thus powered by IPLD's data models and the libp2p for network communication. Kubo is written in Go.

Featureset
- Runs an IPFS-Node as a network service that is part of LAN and WAN DHT
- [HTTP Gateway](https://specs.ipfs.tech/http-gateways/) (`/ipfs` and `/ipns`) functionality for trusted and [trustless](https://docs.ipfs.tech/reference/http/gateway/#trustless-verifiable-retrieval) content retrieval
- [HTTP Routing V1](https://specs.ipfs.tech/routing/http-routing-v1/) (`/routing/v1`) client and server implementation for [delegated routing](./docs/delegated-routing.md) lookups
- [HTTP Kubo RPC API](https://docs.ipfs.tech/reference/kubo/rpc/) (`/api/v0`) to access and control the daemon
- [Command Line Interface](https://docs.ipfs.tech/reference/kubo/cli/) based on (`/api/v0`) RPC API
- [WebUI](https://github.com/ipfs/ipfs-webui/#readme) to manage the Kubo node
- [Content blocking](/docs/content-blocking.md) support for operators of public nodes

### Other implementations

See [List](https://docs.ipfs.tech/basics/ipfs-implementations/)

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
- [Minimal System Requirements](#minimal-system-requirements)
- [Install](#install)
  - [Docker](#docker)
  - [Official prebuilt binaries](#official-prebuilt-binaries)
    - [Updating](#updating)
      - [Using ipfs-update](#using-ipfs-update)
      - [Downloading builds using IPFS](#downloading-builds-using-ipfs)
  - [Unofficial Linux packages](#unofficial-linux-packages)
    - [ArchLinux](#arch-linux)
    - [Gentoo Linux](#gentoo-linux)
    - [Nix](#nix)
    - [Solus](#solus)
    - [openSUSE](#opensuse)
    - [Guix](#guix)
    - [Snap](#snap)
    - [Ubuntu PPA](#ubuntu-ppa)
    - [Fedora](#fedora-copr)
  - [Unofficial Windows packages](#unofficial-windows-packages)
    - [Chocolatey](#chocolatey)
    - [Scoop](#scoop)
  - [Unofficial MacOS packages](#unofficial-macos-packages)
    - [MacPorts](#macports)
    - [Nix](#nix-macos)
    - [Homebrew](#homebrew)
  - [Build from Source](#build-from-source)
    - [Install Go](#install-go)
    - [Download and Compile IPFS](#download-and-compile-ipfs)
      - [Cross Compiling](#cross-compiling)
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

### Minimal System Requirements

IPFS can run on most Linux, macOS, and Windows systems. We recommend running it on a machine with at least 6 GB of RAM and 2 CPU cores (ideally more, Kubo is highly parallel).

> [!CAUTION]
> On systems with less memory, it may not be completely stable, and you run on your own risk.

## Install

The canonical download instructions for IPFS are over at: https://docs.ipfs.tech/install/. It is **highly recommended** you follow those instructions if you are not interested in working on IPFS development.

### Docker

Official images are published at https://hub.docker.com/r/ipfs/kubo/: [![Docker Image Version (latest semver)](https://img.shields.io/docker/v/ipfs/kubo?color=blue&label=kubo%20docker%20image&logo=docker&sort=semver&style=flat-square&cacheSeconds=3600)](https://hub.docker.com/r/ipfs/kubo/)

#### 🟢 Release Images
  - These are production grade images. Use them.
  - `latest` and [`release`](https://hub.docker.com/r/ipfs/kubo/tags?name=release) tags always point at [the latest stable release](https://github.com/ipfs/kubo/releases/latest). If you use this, remember to `docker pull` periodically to update.
  - [`vN.N.N`](https://hub.docker.com/r/ipfs/kubo/tags?name=v) points at a specific [release tag](https://github.com/ipfs/kubo/releases)

#### 🟠 Developer Preview Images
  - These tags are used by developers for internal testing, not intended for end users or production use.
  - [`master-latest`](https://hub.docker.com/r/ipfs/kubo/tags?name=master-latest) always points at the `HEAD` of the [`master`](https://github.com/ipfs/kubo/commits/master/) branch
  - [`master-YYYY-DD-MM-GITSHA`](https://hub.docker.com/r/ipfs/kubo/tags?name=master-2) points at a specific commit from the `master` branch

#### 🔴 Internal Staging Images
  - We use `staging` for testing arbitrary commits and experimental patches.
    - To build image for current HEAD, force push to `staging` via  `git push origin HEAD:staging --force`)
  - [`staging-latest`](https://hub.docker.com/r/ipfs/kubo/tags?name=staging-latest) always points at the `HEAD` of the [`staging`](https://github.com/ipfs/kubo/commits/staging/) branch
  - [`staging-YYYY-DD-MM-GITSHA`](https://hub.docker.com/r/ipfs/kubo/tags?name=staging-2) points at a specific commit from the `staging` branch

```console
$ docker pull ipfs/kubo:latest
$ docker run --rm -it --net=host ipfs/kubo:latest
```

To [customize your node](https://docs.ipfs.tech/install/run-ipfs-inside-docker/#customizing-your-node),
pass necessary config via `-e` or by mounting scripts in the `/container-init.d`.

Learn more at https://docs.ipfs.tech/install/run-ipfs-inside-docker/

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

<a href="https://repology.org/project/kubo/versions">
    <img src="https://repology.org/badge/vertical-allrepos/kubo.svg" alt="Packaging status" align="right">
</a>

- [ArchLinux](#arch-linux)
- [Gentoo Linux](#gentoo-linux)
- [Nix](#nix-linux)
- [Solus](#solus)
- [openSUSE](#opensuse)
- [Guix](#guix)
- [Snap](#snap)
- [Ubuntu PPA](#ubuntu-ppa)
- [Fedora](#fedora-copr)

#### Arch Linux

[![kubo via Community Repo](https://img.shields.io/archlinux/v/community/x86_64/kubo?color=1793d1&label=kubo&logo=arch-linux&style=flat-square&cacheSeconds=3600)](https://wiki.archlinux.org/title/IPFS)

```bash
# pacman -S kubo
```

[![kubo-git via AUR](https://img.shields.io/static/v1?label=kubo-git&message=latest%40master&color=1793d1&logo=arch-linux&style=flat-square&cacheSeconds=3600)](https://aur.archlinux.org/packages/kubo/)

#### <a name="gentoo-linux">Gentoo Linux</a>

https://wiki.gentoo.org/wiki/Kubo

```bash
# emerge -a net-p2p/kubo
```

https://packages.gentoo.org/packages/net-p2p/kubo

#### <a name="nix-linux">Nix</a>

With the purely functional package manager [Nix](https://nixos.org/nix/) you can install kubo (go-ipfs) like this:

```
$ nix-env -i kubo
```

You can also install the Package by using its attribute name, which is also `kubo`.

#### Solus

[Package for Solus](https://dev.getsol.us/source/kubo/repository/master/)

```
$ sudo eopkg install kubo
```

You can also install it through the Solus software center.

#### openSUSE

[Community Package for go-ipfs](https://software.opensuse.org/package/go-ipfs)

#### Guix

[Community Package for go-ipfs](https://packages.guix.gnu.org/packages/go-ipfs/0.11.0/) is now out-of-date.

#### Snap

No longer supported, see rationale in [kubo#8688](https://github.com/ipfs/kubo/issues/8688).

#### Ubuntu PPA

[PPA homepage](https://launchpad.net/~twdragon/+archive/ubuntu/ipfs) on Launchpad.

##### Latest Ubuntu (>= 20.04 LTS)
```sh
sudo add-apt-repository ppa:twdragon/ipfs
sudo apt update
sudo apt install ipfs-kubo
```

### Fedora COPR

[`taw00/ipfs-rpm`](https://github.com/taw00/ipfs-rpm)

##### Any Ubuntu version

```sh
sudo su
echo 'deb https://ppa.launchpadcontent.net/twdragon/ipfs/ubuntu <<DISTRO>> main' >> /etc/apt/sources.list.d/ipfs
echo 'deb-src https://ppa.launchpadcontent.net/twdragon/ipfs/ubuntu <<DISTRO>> main' >> /etc/apt/sources.list.d/ipfs
exit
sudo apt update
sudo apt install ipfs-kubo
```
where `<<DISTRO>>` is the codename of your Ubuntu distribution (for example, `jammy` for 22.04 LTS). During the first installation the package maintenance script may automatically ask you about which networking profile, CPU accounting model, and/or existing node configuration file you want to use.

**NOTE**: this method also may work with any compatible Debian-based distro which has `libc6` inside, and APT as a package manager.

### Unofficial Windows packages

- [Chocolatey](#chocolatey)
- [Scoop](#scoop)

#### Chocolatey

No longer supported, see rationale in [kubo#9341](https://github.com/ipfs/kubo/issues/9341).

#### Scoop

Scoop provides kubo as `kubo` in its 'extras' bucket.

```Powershell
PS> scoop bucket add extras
PS> scoop install kubo
```

### Unofficial macOS packages

- [MacPorts](#macports)
- [Nix](#nix-macos)
- [Homebrew](#homebrew)

#### MacPorts

The package [ipfs](https://ports.macports.org/port/ipfs) currently points to kubo (go-ipfs) and is being maintained.

```
$ sudo port install ipfs
```

#### <a name="nix-macos">Nix</a>

In macOS you can use the purely functional package manager [Nix](https://nixos.org/nix/):

```
$ nix-env -i kubo
```

You can also install the Package by using its attribute name, which is also `kubo`.

#### Homebrew

A Homebrew formula [ipfs](https://formulae.brew.sh/formula/ipfs) is maintained too.

```
$ brew install --formula ipfs
```

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

### HTTP/RPC clients

For programmatic interaction with Kubo, see our [list of HTTP/RPC clients](docs/http-rpc-clients.md).

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

Kubo is maintained by [Shipyard](https://ipshipyard.com/).

* This repository is part of [Shipyard's GO Triage triage](https://ipshipyard.notion.site/IPFS-Go-Triage-Boxo-Kubo-Rainbow-0ddee6b7f28d412da7dabe4f9107c29a).
* [Release Process](https://ipshipyard.notion.site/Kubo-Release-Process-6dba4f5755c9458ab5685eeb28173778)


## Contributing

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

We ❤️ all [our contributors](docs/AUTHORS); this project wouldn’t be what it is without you! If you want to help out, please see [CONTRIBUTING.md](CONTRIBUTING.md).

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

Members of IPFS community provide Kubo support on [discussion forum category here](https://discuss.ipfs.tech/c/help/help-kubo/23).

Need help with IPFS itself? Learn where to get help and support at https://ipfs.tech/help.

## License

This project is dual-licensed under Apache 2.0 and MIT terms:

- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/ipfs/kubo/blob/master/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](https://github.com/ipfs/kubo/blob/master/LICENSE-MIT) or http://opensource.org/licenses/MIT)
