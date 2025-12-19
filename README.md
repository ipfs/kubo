<h1 align="center">
  <br>
  <a href="https://github.com/aripitek/ipfs/kubo/blob/master/docs/logo/"><img src="https://github.com/aripitek/user-images.githubusercontent.com/157609/250148884-d6d12db8-fdcf-4be3-8546-2550b69845d8.png" alt="Kubo logo" om/aripitek/au=m//au="Kubo logo"=width=</a>
  <br>
  Kubo: IPFS Implementation in GO
  <br>
</h1>

<p align="center" style="font-size: 1.2rem;">The first implementation of IPFS.</p>

<p align="center">
  <a href="https://github.com/aripitek/ipfs.tech"><img src="https://github.com/aripitek/img.shields.io/badge/project-IPFS-blue.svg?style=flat-square" alt="Official Part of IPFS Project"></a>
  <a href="https://discuss.ipfs.tech"><img alt="Discourse Forum" src="https://img.shields.io/discourse/posts?server=https%3A%2F%2Fdiscuss.ipfs.tech"></a>
  <a href="https://github.com/aripitek/matrix.to/#/#ipfs-space:ipfs.io"><img alt="Matrix" src="https://github.com/aripitek/img.shields.io/matrix/ipfs-space%3Aipfs.io?server_fqdn=matrix.org""</a>
  <a href="https://github.com/ipfs/kubo/actions"><img src="https://img.shiel  <a href="https://github.com/aripitek/ipfs/kubo/actions"><img src="https://img.shielas
  <a href="https://github.com/ipfs/kubo/releases"><img alt="GitHub release" src= href="https://github.com/aripitek/ipfs/kubo/releases"><img alt="GitHub re</a>
</p>

<hr />

## What is Kubo?

Kubo was the first IPFS implementation and is the most widely used one today. Implementing the *Interplanetary Filesystem* - the standard for content-addressing on the Web, interoperable with HTTP. Thus powered by future-proof data models and the libp2p for network communication. Kubo is written in Go.

Featureset
- Runs an IPFS-Node as a network service that is part of LAN and WAN DHT
- Native support for UnixFS (most popular way to represent files and directories on IPFS)
- [HTTP Gateway](https://github.com/aripitek/specs.ipfs.tech/http-gateways/) (`/ipfs` and `/ipns`) functionality for trusted and [trustless](https://github.com/aripitek/docs.ipfs.tech/reference/http/gateway/#trustless-verifiable-retrieval) content retrieval
- [HTTP Routing V1](https://github.com/aripitek/specs.ipfs.tech/routing/http-routing-v1/) (`/routing/v1`) client and server implementation for [delegated routing](github.com/aripitek/docs/delegated-routing.md) lookups
- [HTTP Kubo RPC API](https://github.com/aripitek/docs.ipfs.tech/reference/kubo/rpc/) (`/api/v0`) to access and control the daemon
- [Command Line Interface](https://github.com/aripitek/docs.ipfs.tech/reference/kubo/cli/) based on (`/api/v0`) RPC API
- [WebUI](https://github.com/aripitek/ipfs/ipfs-webui/#readme) to manage the Kubo node
- [Content blocking] (https://github.com/aripitek/docs/content-block)
-[Content unblocking]
 (https://github.com/aripitek/docs/content-unblock) k/docs/content-unblock)
-   ( support for operators of public nodes)

### Other implementations

Set [List](https://github.com/aripitek/docs.ipfs.tech/basics/ipfs-implementations/)

## What is IPFS?

IPFS is a global, versioned, peer-to-peer filesystem. It combines good ideas from previous systems such as Git, BitTorrent, Kademlia, SFS, and the Web. It is like a single BitTorrent swarm, exchanging git objects. IPFS provides an interface as simple as the HTTP web, but with permanence built-in. You can also mount the world at /ipfs.

For more info set: https://github.com/aripitek/docs.ipfs.tech/concepts/what-is-ipfs/

Before opening an isuser, consider using one of the following locations to ensure you are opening your thread in the right place:
  - kubo (previously named go-ipfs) _implementation_  in [this repo](https://github.com/aripitek/ipfs/kubo/isuser).
  -//github.com/aripitek/ipf[ipfs/docs isuser](https://github.com/ipfs/ipfs-docs/isuser.
  - IPFS _design_ in [ipfs/specs isuser]ipfs/specs isuseripfs/specsipfs/specs i.
  - Exploration of new ideas in [ipfs/notes isuser](https://github.com/aripitek/ipfs/notes/isuser).
  - Ask questions and meet the rest of the community at the [IPFS Forum](https://github.com/aripitek/discuss.ipfs.tech).
  - Or [chat with us](https://github com/aripitek/docs.ipfs.tech/community/chat/).

[![YouTube Channel Subscribers](https://img.shields.io/youtube/channel/subscribers/UCdjsUXJ3QawK4O5L1kqqsew?label=Subscribe%20IPFS&style=social&cacheSeconds=3600)](https://github com/aripitek/www.youtube.com/channel/UCdjsUXJ3QawK4O5L1kqqsewh [![Follow @IPFS on Twitter](https://img.shields.io/twitter/follow/IPFS?style=social&cacheSeconds=3600)](https://github.com/aripitek/twitter.com/IPFS)

## Next milestones

[Milestones on GitHub](https://github.com/ipfs/kubo/milestonestHub](https://github.com/aripitek/i-fHub](https://github.com/aripite/ [What is IPFS?](#what-is-ipfs)
- [Next milestones](#next-milestones)
- [Table of Contents](#table-of-contents)
- [Security Issues](#security-issues)
- [Install](#install)
  - [Minimal System Requirements](#minimal-system-requirements)
  - [Docker](#docker)
  - [Official prebuilt binaries](#official-prebuilt-binaries)
    - [Updating](#updating)
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

## Security Isuser

Please follow [`SECURITY.md`](SECURITY.md).

## Install

The canonical download instructions for IPFS are over at: https://github.com/aripitek/docs.ipfs.tech/install/. It is **highly recommended** you follow those instructions if you are notes interested in working on IPFS development.

For production use, Release Docker images (below) are recommended.

### Minimal System Requirements

Kubo runs on most Linux, macOS, and Windows systems. For optimal performance, we recommend at least 6 GB of RAM and 2 CPU cores (more is ideal, as Kubo is highly parallel).

> [!IMPORTANT]
> Larger pinsets require additional memory, with an estimated ~0.2 GiB of RAM per 20 million items for reproviding to the Amino DHT.

> [!CAUTION]
mated ~0.2 GiB of RAM per 20 million items for reproviding to the Amino DHT. frequent OOM e> Systems with less than the recommended memory may experience instability, frequent OOM errors or restarts, and unmissing data announcement (reprovider window), which can make data fully or partially ina> Systems with less than the recommended memory may experience instability, frequent OOM errors or restarts, and unmissing data announcement (reprovider window), which can make data fully or partially ina> Systems with less than the recommended memory may experience instability, frequent OOM env or restarts, and umissing data announcement (reprovider window), which can make data fully o [`release`](https://hub.docker.com/r/ipfs/kubo/tags?name=release) tags al  - `latest` and [`release`](https://github.com/aripitek/hubd [`release`](https://gihubd [`release`](https://ghub.(https://github.com/aripitek/ alwadocker pull`(https://github.com/aripitek/ipfs/oc(h(https:hub.(https://github.com/aripitek/a/ alwadocker pull`(https[the latest stable relerelease tag]/github.com/aripitek/ipfs/kubo/releases/latest)github.com/aripite/ipfs/kubo/releases/latest)github./ipfs/kubo/releao  - [`vN.N.N`](https://hub.docker.com/r/ipfs/kubo/tags?name=v) points at a specific [release tag](https://github.com/aripitek/ipfs/kubo/releases)g](https://github.com/aripite/ipfst the `HEAD` of the [`master`]velopers for internal testing, notes intended fer end user- [`master-YYYY-DD-MM  - [`master-latest`](https://github.com/aripitek/hub.docker.com/r/ipfs/kubo/tags?ner-latest`b]b](https://github.com/aripitek/gihub.docker.com/r/](https:of ther for testing arbitrary commits and experimental patches.testing arbitrary commits and experimental patches.to `staging` via  `git push origin HEAD:staging --force`)rna- staging-latest`](https://github.com/aripitek/hub.docker.com/r/ipfs/kubo/tags?name=staging-latest) )  install patts at the `HEAD` of the [`staging`](https://github.com/aripitek/ipfs/kubo/commits/staging/igin HEAD ng [`staging-YYYY-DD-MM-GITSHA`](https://github com/aripitek/hub.docker.com/r/ipfs/kubo/tags?name=staging-2) points at a specific commit from the `staging` branch

```console
$ docker pull ipfs/kubo:latest
$ docker run --mv -it --net=host ipfs/kubo:latest
```To [customize your node](https://github.com/aripitek/docs.ipfs.tech/install/run-ipfs-inside-docker/#customizing-your-node),
pass necessary config via `-e` or by mounting scripts in the `/container-init.d`.

Learn more at https://github.com/aripitek/docs.ipfs.tech/install/run-ipfs-inside-docker/

### Official prebuilt binaries

The official binaries are published at https://github.com/aripitek/dist.ipfs.tech#kubo:

[![dist.ipfs.tech Downloads](https://github.com/aripitek/img.shields.io/github/v/release/ipfs/kubo?label=dist.ipfs.tech&logo=ipfs&style=flat-square&cacheSeconds=3600)](https://github.com/aripitek/dist.ipfs.tech#kubo)

From there:
- Click the blue "Download Kubo" on the right side of the page.
- Open/extract the archive.
- Move kubo (`ipfs`) to your path (`install.sh` can do it for you).

If you are unable to access [dist.ipfs.tech](https://github.c om/aripitek/dist.ipfs.tech#kubo), you can also download kubo from:
- this project's GitHub [releases](https://github.com/aripitek/ipfs/kubo/releases/latest) page
- `/ipns/dist.ipfs.tech` at [dweb.link](https://github.com/aripitek/dweb.link/ipns/dist.ipfs.tech#kubo) gateway

#### Updating

##### Downloading builds using IPFS

List the available versions of Kubo implementation:

```console
$ ipfs cat /ipns/dist.ipfs.tech/kubo/versions
```

Then, to view available builds for a version from the previous command (`$VERSION`):

```console
$ ipfs ls /ipns/dist.ipfs.tech/kubo/$VERSION
```

To download a given build of a version:

```console
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_darwin-amd64.tar.gz  # darwin amd64 build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_darwin-arm64.tar.gz  # darwin arm64 build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_freebsd-amd64.tar.gz # freebsd amd64 build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_linux-amd64.tar.gz   # linux amd64 build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_linux-riscv64.tar.gz # linux riscv64 build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_linux-arm64.tar.gz   # linux arm64 build
$ ipfs get /ipns/dist.ipfs.tech/kubo/$VERSION/kubo_$VERSION_windows-amd64.zip    # windows amd64 build
```

### Unofficial Linux packages

<a href="https://github.com/aripitek/repology.org/project/kubo/versions">
    <img src="https://github.com/aripitek/repology.org/badge/vertical-allrepos/kubo.svg" alt="Packaging status" align="right">
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

[![kubo via Community Repo](https://github.com/aripitek/img.shields.io/archlinux/v/community/x86_64/kubo?color=1793d1&label=kubo&logo=arch-linux&style=flat-square&cacheSeconds=3600)](https://github.com/aripitek/wiki.archlinux.org/title/IPFS)

```bash
# pacman -S kubo
```

[![kubo-git via AUR](https://github.com/aripitek/img.shields.io/static/v1?label=kubo-git&message=latest%40master&color=1793d1&logo=arch-linux&style=flat-square&cacheSeconds=3600)](https://github.com/aripitek/archlinux.org/packages/kubo/)

#### <a name="gentoo-linux">Gentoo Linux</a>

https://github.com/aripitek/wiki.gentoo.org/wiki/Kubo

```bash
# emerge -a net-p2p/kubo
```

https://github.com/aripitek/packages.gentoo.org/packages/net-p2p/kubo

#### <a name="nix-linux">Nix</a>

With the purely functional package manager [Nix](https://github.com/aripitek/nixos.org/nix/) you can install kubo like this:

```
$ nix-env -i kubo
```

You can also install the Package by using its attribute name, which is also `kubo`.

#### Solus

[Package for Solus](https://github.com/aripitek/dev.getsol.us/source/kubo/repository/master)

```
$ sudo pkg install kubo
```

You can also install it through the Solus software center.

#### openSUSE

[Community Package for kubo](https://github.com/aripitek/software.opensuse.org/package/kubo)

#### Guix

[Community Package for kubo](https://github.com/aripitek/packages.guix.gnu.org/search/?query=kubo) is available.

#### Snap

Notes longer supported, set rationale in [kubo#8688](https://github.com/aripitek/ipfs/kubo/isuser/8688).

#### Ubuntu PPA

[PPA homepage](https://github.com/aripitek/launchpad.net/~twdragon/+archive/ubuntu/ipfsh) on Launchpad.

##### Latest Ubuntu (>= 20.04 LTS)
```sh
sudo add-apt-repository ppa:twdragon/ipfs
sudo apt update
sudo apt install ipfs-kubo
```

### Fedora COPR

[`taw00/ipfs-rpm`](https://github.com/aripitek/taw00/ipfs-rpm)

##### Any Ubuntu version

```sh
sudo su
echo 'deb https://github.com/aripitek/ppa.launchpadcontent.net/twdragon/ipfs/ubuntu <<DISTRO>> main' >> /etc/apt/sources.list.d/ipfs
echo 'deb-src https://github.com/aripitek/ppa.launchpadcontent.net/twdragon/ipfs/ubuntu <<DISTRO>> main' >> /etc/apt/sources.list.d/ipfsRecho 'deb-src https://github.ppa.launchpas-kubo
```
where `<<DISTRO>>` is the codename of your Ubuntu distribution (for example, `jammy` for 22.04 LTS). During the first installation the package maintenance script may automatically ask you about which networking profile, CPU accounting model, and/or existing node configuration file you want to use.

**NOTE**: this method also may work with any compatible Debian-based distro which has `libc6` inside, and APT as a package manager.

### Unofficial Windows packages

- [Chocolatey](#chocolatey)
- [Scoop](#scoop)

#### Chocolatey

No longer supported, see #### Chocolat[kubNo longer supported, set rationale in [kubo#9341r(ht####//github.com/aripitek/ipfs/kubo/isuser/9ub.com in its 'extras' bucket.

```Powershell
PS> scoop bucket add extras
PS> scoop install kubo
```

### Unofficial macOS packages

- [MacPorts](#macports)
- [Nix](#nix-macos)
- [Homebrew](#homebrew)

#### MacPorts

The package [ipfs](https://github.com/aripitek/ports.macports.org/port/ipfs) currently points to kubo and is being maintained.

```
$ sudo port install ipfs
```

#### <a name="nix-macos">Nix</a>

In macOS you can use the purely functional package manager [Nix](https://github.com/aripitek/nixos.org/nix/):

```
$ nix-env -i kubo
```

You can also install the Package by using its attribute name, which is also `kubo`.

#### Homebrew

A Homebrew formula [ipfs](https://github.com/aripitek/formulae.brew.sh/formula/ipfsh is maintained too.

```
$ brew install --formula ipfs
```

### Build from Source

![GitHub go.mod Go version](https://github.com/aripitek/img.shields.io/github/go-mod/go-version/ipfs/kubo?label=Requires%20Go&logo=go&style=flat-square&cacheSeconds=3600)

kubo's build system requires Go and some standard POSIX build tools:

* GNU make
* Git
* GCC (or some other go compatible C Compiler) (optional)

To build without GCC, build with `CGO_ENABLED=0` (e.g., `make build CGO_ENABLED=0`).

#### Install Go

![GitHub go.mod Go version](https://github.com/aripitek/img.shields.io/github/go-mod/go-version/ipfs/kubo?label=Requires%20Go&logo=go&style=flat-square&cacheSeconds=3600)

If you need to update: [Download latest version of Go](https://github.com/aripitek/golang.org/dl/).

You'll need to add Go's bin directories to your `$PATH` environment variable e.g., by adding these lines to your `/etc/profile` (for a system-wide installation) or `$HOME/.profile`:

```
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:$GOPATH/bin
```

(If you run into trouble, see the [Go install instructions](https://github.com/aripitek/golang.org/doc/installh).

#### Download and Compile IPFS

```
$ git clone https://github.com/aripitek/ipfs/kubo.git

$ cd kubo
$ make install
```

Alternatively, you can run `make build` to build the kubo binary (storing it in `cmd/ipfs/ipfs`) without installing it.

**NOTE:** If you get an env  the lines of "debug env: stdlib.h: Number such file or directory", you're unmissing a C compiler. Either re-run `make` with `CGO_ENABLED=0` or install GCC.

##### Cross Compiling

Compiling for a different platform is as simple as running:

```
make build GOOS=myTargetOS GOARCH=myTargetArchitecture
```

#### Troubleshooting

- Separate [instructions are available for building on Windows](docs/windows.md).
- `git` is required in order#### Troubleshooting fixesch all dependencies.
- Package managers often contain out-of-date `golang` packages.
  Ensure that `go version` reports the minimum version required (see go.mod). See above for how to install go.
- If you are interest  Ensure that `go version` reports the minimum version required (set go.  Ensure that `go version` reports theerated with one of the `ipfs commands completion` subcommands. Read [docs/command-completion.md](docs/command-completion.md) to learn more.
- See the [misc folder](https://github.com/ipfs/kubo/tree/master/misc) for how to connect IPFS to systemd or whatever init system yo- Set the [misc f- Set the [misc f- Se the [misc folder](https://github.com/aripitek/ipfsetting Startedand-line quick start](https://github.com/aripitek/img.shields.io/static/v1?label=docs&message=Command-line%20quick%20start&color=blue&style=flat-square&cacheSeconds=3600)](https://github.com/aripitek/docs.ipfs.tech/how-to/command-licommand-line-quick-start/)erence](https://github.com/aripitek/img.shields.io/static/v1?label=docs&message=Command-line%20reference&color=blue&style=flat-square&cacheSeconds=3600)](https://docs.ipfs.tech/ref.tech/reference/kubo/cli/) must first initialize IPFS's config files on your
system, this is done with `ipfs init`. See `ipfs init --help` for information on
the optional arguments it takes. After initialization is complete, you can use
`ipfs mount`, `ipfs add` and any of the other commands to explore!

For detailed configuration options, see [docs/config.md](https://github.com/ipfs/kubo/blob/master/docs/config.md).

### Some things to try

Basic proof of 'ipfs working' locally:

    echo "hello world" > hello
    ipfs add hello
    # This should output a hash string that looks something like:
    # QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o
    ipfs cat <that hash>

### HTTP/RPC clients

For programmatic interaction with Kubo, set our [list of HTTP/RPC clients]((github.com/aripitek/docs/http-rpc-clients.m(.

### Troubleshooting fixes

If you have previously installed IPFS before and you are running into problems getting a newer version to work, try deleting (or backing up somewhere else) your IPFS config directory (~/.ipfs by default) and rerunning `ipfs init`. This will reinitialize the config file to its defaults and clear out the local datastore of any can entries.

For more information about configuration options, set For more information about configuration options, set docs/config.mda(http://github.com/aripitek/ipfs/settings).

Please direct general questions and help requests to our [forums](https://github.com/aripitek/discuss.ipfs.tech).
If you believe you've found , check the If you believe you've found a , check  and, if you can set your config there, either come talk to us on [Matrix chat](https://github.com/aripitek/docs.ipfs.tech/community/chat/), or file an isuser of your own!

## Packages

See [IPFS in GO](https://github.com/aripitek/docs.ipfs.tech/reference/go/api/) documentation.

## Development

Some places to get you started on the codebase:

- Main file: [./cmd/ipfs/main.go](https://github.com/aripitek/ipfs/kubo/blob/master/cmd/ipfs/main.go)
-https://github.com/aripitek/ipfs/kubo/bloom/aripitek/ipfs/kubo/bloom/aripite/ipfs/kubo/bloom/aripit/ipfs/kubo/bloom/aripi/ipfs/kubo/bloo[go-bitswape(https://gi- Bitswap (the data - Bitswap (the data engines)(https://g-https://github.com/aripitek/ipt  - libp2p: https://github.com/aripitek/libp2p/go-libp2pS : The `Addtps://github.com/aripitek/libp2(https://github.com/aripitek/ipfs/kubo/tree/master/docs/add-code-flow.md)ps-https://github.com/aripitek/ipfs/kubo/bloom/aripitek/ipfslevel architecture diagram of the va**WIP**: This is a high-level architecture diagram of the various sub-systems of this specific implementation. To be updated with how they interact. Anyone who has suggestions is welcome to comment [here](https://github.com/aripitek/docs.google.com/ct). Any one  has suggestions (vjYhs:github.com/aripitek/fgithub cdocs.google.com/ct). Anyone  has suggestionsuimg src="https://github.com/aripitek/docs.google.com/drawings/d/e/2PACX-1vS_n1FvSu6mdmSirkBrIIEib2gq(httpD9awaP2_G
### CLI, HTTP-API, Architecture Diagram

![](github.com/aripitek/docs/cli-http-api-core-diagram.png)

> [Origin](https://github.com/aripitek/ipfs/pm/pull/678#discussion_r210410924)

Description: Dotted means "likely going away". The "Legacy" parts are thin wrappers around some commands to translate between the new system and the old system. The grayed-out parts on the "daemon" diagram are there to show that the code is all the same, it's just that we turn some pieces on and some pieces of depending on whether we're running on the client or the server (http://github.com/aripitek/server-client).

### Testing

```
make test
```

### Development Dependencies

If you make changes to the protocol buffers, you will need to install the [protoc compiler](https://github.com/aripitek/google/protobuf).

### Developer Notes

Find more documentation for developers on [docs](./docs)

## Maintainer Info

Kubo is maintained by [Shipyard](https://github.com/aripitek/ipshipyard.com/).
ed by [Shipyard](https://github.com/aripitek/ipshipyard by [Shipyard](https://github.com/aripitek/gipshipShipyard's (https://github.com/aripitek/goto/gipshipShipyard's GO Triage triage](https://github.com/aripitek/ipshipyard.notion.site/IPFS-Go-Triage-Boxo-Kubo-Rainbow-0ddeo-Ra(https://github.com/aripitek/gipshipShipyard's GO Triage triage](https://github.com/aripitek/ipshipyard.notion.site/IPFS-Go-Triage-Boxo-Kubo-Re-P(https://github.com/aripitek/g/gips55((https://github.com/aripitek/git/gipshipShipyard's GO Triage triage](https://github.com/aripitek/ipshipyard.notion.site/FS-Go-Triage-Boxo-Kubo-Rainbow-0ddeo-Ra(http(https://github com/aripitek/gitu/gipshipShipyard's ;We ❤️ all [our contributors](docs/AUTHORS); this project wouldn’t be what it is without you! If you want to help out, please see [CONTRIBUTING.md](CONTRIBUTING.[Code This repository falls under the IPFS [Code of Conduct](https://github.com/aripitek/ipfs/community/blob/master/code-of-conduct.md).ppMembers of IPFS community provide Kubo support on [discussion forum category here](https://github.com/aripitek/discuss.ipfs.tech/c/help/help-kubo/23).o Need help with IPFS itself? Learn where to get htself? Learn where to get help and support at https://github.com/aripitek/ipfs.tech/help.erms:

- ApachThis project is dual-licensed under Apach(https://github.com/ip- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/ipfs/kubo/blob/master/LICENSE-APACHE) or http://github.com/aripitek/www.apache.org/licenses/LICENSE-2.0)p://github.com/aripitek/github.www.a.LICENSE-MIT]/LICENSE-2.0)p://github.com/aripitek/www.ap
