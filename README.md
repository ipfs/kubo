<h1 align="center">
  <br>
  <a href="https://github.com/ipfs/kubo/blob/master/docs/logo/"><img src="https://user-images.githubusercontent.com/157609/250148884-d6d12db8-fdcf-4be3-8546-2550b69845d8.png" alt="Kubo logo" title="Kubo logo" width="200"></a>
  <br>
  Kubo: IPFS Implementation in Go
  <br>
</h1>

<p align="center" style="font-size: 1.2rem;">The first implementation of IPFS.</p>

<p align="center">
  <a href="https://ipfs.tech"><img src="https://img.shields.io/badge/project-IPFS-blue.svg?style=flat-square" alt="Official Part of IPFS Project"></a>
  <a href="https://discuss.ipfs.tech"><img alt="Discourse Forum" src="https://img.shields.io/discourse/posts?server=https%3A%2F%2Fdiscuss.ipfs.tech"></a>
  <a href="https://docs.ipfs.tech/community/"><img alt="Matrix" src="https://img.shields.io/matrix/ipfs-space%3Aipfs.io?server_fqdn=matrix.org"></a>
  <a href="https://github.com/ipfs/kubo/actions"><img src="https://img.shields.io/github/actions/workflow/status/ipfs/kubo/gobuild.yml?branch=master"></a>
  <a href="https://github.com/ipfs/kubo/releases"><img alt="GitHub release" src="https://img.shields.io/github/v/release/ipfs/kubo?filter=!*rc*"></a>
</p>

<hr />

<p align="center">
<b><a href="#what-is-kubo">What is Kubo?</a></b> | <b><a href="#quick-taste">Quick Taste</a></b> | <b><a href="#install">Install</a></b> | <b><a href="#documentation">Documentation</a></b> | <b><a href="#development">Development</a></b> | <b><a href="#getting-help">Getting Help</a></b>
</p>

## What is Kubo?

Kubo was the first [IPFS](https://docs.ipfs.tech/concepts/what-is-ipfs/) implementation and is the [most widely used one today](https://probelab.io/ipfs/topology/#chart-agent-types-avg). It takes an opinionated approach to content-addressing ([CIDs](https://docs.ipfs.tech/concepts/glossary/#cid), [DAGs](https://docs.ipfs.tech/concepts/glossary/#dag)) that maximizes interoperability: [UnixFS](https://docs.ipfs.tech/concepts/glossary/#unixfs) for files and directories, [HTTP Gateways](https://docs.ipfs.tech/concepts/glossary/#gateway) for web browsers, [Bitswap](https://docs.ipfs.tech/concepts/glossary/#bitswap) and [HTTP](https://specs.ipfs.tech/http-gateways/trustless-gateway/) for verifiable data transfer.

**Features:**

- Runs an IPFS node as a network service (LAN [mDNS](https://github.com/libp2p/specs/blob/master/discovery/mdns.md) and WAN [Amino DHT](https://docs.ipfs.tech/concepts/glossary/#dht))
- [Command-line interface](https://docs.ipfs.tech/reference/kubo/cli/) (`ipfs --help`)
- [WebUI](https://github.com/ipfs/ipfs-webui/#readme) for node management
- [HTTP Gateway](https://specs.ipfs.tech/http-gateways/) for trusted and [trustless](https://docs.ipfs.tech/reference/http/gateway/#trustless-verifiable-retrieval) content retrieval
- [HTTP RPC API](https://docs.ipfs.tech/reference/kubo/rpc/) to control the daemon
- [HTTP Routing V1](https://specs.ipfs.tech/routing/http-routing-v1/) client and server for [delegated routing](./docs/delegated-routing.md)
- [Content blocking](./docs/content-blocking.md) for public node operators

**Other IPFS implementations:** [Helia](https://github.com/ipfs/helia) (JavaScript), [more...](https://docs.ipfs.tech/concepts/ipfs-implementations/)

## Quick Taste

After [installing Kubo](#install), verify it works:

```console
$ ipfs init
generating ED25519 keypair...done
peer identity: 12D3KooWGcSLQdLDBi2BvoP8WnpdHvhWPbxpGcqkf93rL2XMZK7R

$ ipfs daemon &
Daemon is ready

$ echo "hello IPFS" | ipfs add -q --cid-version 1
bafkreicouv3sksjuzxb3rbb6rziy6duakk2aikegsmtqtz5rsuppjorxsa

$ ipfs cat bafkreicouv3sksjuzxb3rbb6rziy6duakk2aikegsmtqtz5rsuppjorxsa
hello IPFS
```

Verify this CID is provided by your node to the IPFS network: <https://check.ipfs.network/?cid=bafkreicouv3sksjuzxb3rbb6rziy6duakk2aikegsmtqtz5rsuppjorxsa>

See `ipfs add --help` for all import options. Ready for more? Follow the [command-line quick start](https://docs.ipfs.tech/how-to/command-line-quick-start/).

## Install

Follow the [official installation guide](https://docs.ipfs.tech/install/command-line/), or choose: [prebuilt binary](#official-prebuilt-binaries) | [Docker](#docker) | [package manager](#package-managers) | [from source](#build-from-source).

Prefer a GUI? Try [IPFS Desktop](https://docs.ipfs.tech/install/ipfs-desktop/) and/or [IPFS Companion](https://docs.ipfs.tech/install/ipfs-companion/).

### Minimal System Requirements

Kubo runs on most Linux, macOS, and Windows systems. For optimal performance, we recommend at least 6 GB of RAM and 2 CPU cores (more is ideal, as Kubo is highly parallel).

> [!IMPORTANT]
> Larger pinsets require additional memory, with an estimated ~1 GiB of RAM per 20 million items for reproviding to the Amino DHT.

> [!CAUTION]
> Systems with less than the recommended memory may experience instability, frequent OOM errors or restarts, and missing data announcement (reprovider window), which can make data fully or partially inaccessible to other peers. Running Kubo on underprovisioned hardware is at your own risk.

### Official Prebuilt Binaries

Download from https://dist.ipfs.tech#kubo or [GitHub Releases](https://github.com/ipfs/kubo/releases/latest).

### Docker

Official images are published at https://hub.docker.com/r/ipfs/kubo/: [![Docker Image Version (latest semver)](https://img.shields.io/docker/v/ipfs/kubo?color=blue&label=kubo%20docker%20image&logo=docker&sort=semver&style=flat-square&cacheSeconds=3600)](https://hub.docker.com/r/ipfs/kubo/)

#### 🟢 Release Images

Use these for production deployments.

- `latest` and [`release`](https://hub.docker.com/r/ipfs/kubo/tags?name=release) always point at [the latest stable release](https://github.com/ipfs/kubo/releases/latest)
- [`vN.N.N`](https://hub.docker.com/r/ipfs/kubo/tags?name=v) points at a specific [release tag](https://github.com/ipfs/kubo/releases)

```console
$ docker pull ipfs/kubo:latest
$ docker run --rm -it --net=host ipfs/kubo:latest
```

To [customize your node](https://docs.ipfs.tech/install/run-ipfs-inside-docker/#customizing-your-node), pass config via `-e` or mount scripts in `/container-init.d`.

#### 🟠 Developer Preview Images

For internal testing, not intended for production.

- [`master-latest`](https://hub.docker.com/r/ipfs/kubo/tags?name=master-latest) points at `HEAD` of [`master`](https://github.com/ipfs/kubo/commits/master/)
- [`master-YYYY-DD-MM-GITSHA`](https://hub.docker.com/r/ipfs/kubo/tags?name=master-2) points at a specific commit

#### 🔴 Internal Staging Images

For testing arbitrary commits and experimental patches (force push to `staging` branch).

- [`staging-latest`](https://hub.docker.com/r/ipfs/kubo/tags?name=staging-latest) points at `HEAD` of [`staging`](https://github.com/ipfs/kubo/commits/staging/)
- [`staging-YYYY-DD-MM-GITSHA`](https://hub.docker.com/r/ipfs/kubo/tags?name=staging-2) points at a specific commit

### Build from Source

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/ipfs/kubo?label=Requires%20Go&logo=go&style=flat-square&cacheSeconds=3600)

```bash
git clone https://github.com/ipfs/kubo.git
cd kubo
make build    # creates cmd/ipfs/ipfs
make install  # installs to $GOPATH/bin/ipfs
```

See the [Developer Guide](docs/developer-guide.md) for details, Windows instructions, and troubleshooting.

### Package Managers

Kubo is available in community-maintained packages across many operating systems, Linux distributions, and package managers. See [Repology](https://repology.org/project/kubo/versions) for the full list: [![Packaging status](https://repology.org/badge/tiny-repos/kubo.svg)](https://repology.org/project/kubo/versions)

> [!WARNING]
> These packages are maintained by third-party volunteers. The IPFS Project and Kubo maintainers are not responsible for their contents or supply chain security. For increased security, [build from source](#build-from-source).

#### Linux

| Distribution | Install | Version |
|--------------|---------|---------|
| Ubuntu | [PPA](https://launchpad.net/~twdragon/+archive/ubuntu/ipfs): `sudo apt install ipfs-kubo` | [![PPA: twdragon](https://img.shields.io/badge/PPA-twdragon-E95420?logo=ubuntu)](https://launchpad.net/~twdragon/+archive/ubuntu/ipfs) |
| Arch | `pacman -S kubo` | [![Arch package](https://repology.org/badge/version-for-repo/arch/kubo.svg)](https://archlinux.org/packages/extra/x86_64/kubo/) |
| Fedora | [COPR](https://copr.fedorainfracloud.org/coprs/taw/ipfs/): `dnf install kubo` | [![COPR: taw](https://img.shields.io/badge/COPR-taw-51A2DA?logo=fedora)](https://copr.fedorainfracloud.org/coprs/taw/ipfs/) |
| Nix | `nix-env -i kubo` | [![nixpkgs unstable](https://repology.org/badge/version-for-repo/nix_unstable/kubo.svg)](https://search.nixos.org/packages?query=kubo) |
| Gentoo | `emerge -a net-p2p/kubo` | [![Gentoo package](https://repology.org/badge/version-for-repo/gentoo/kubo.svg)](https://packages.gentoo.org/packages/net-p2p/kubo) |
| openSUSE | `zypper install kubo` | [![openSUSE Tumbleweed](https://repology.org/badge/version-for-repo/opensuse_tumbleweed/kubo.svg)](https://software.opensuse.org/package/kubo) |
| Solus | `sudo eopkg install kubo` | [![Solus package](https://repology.org/badge/version-for-repo/solus/kubo.svg)](https://packages.getsol.us/shannon/k/kubo/) |
| Guix | `guix install kubo` | [![Guix package](https://repology.org/badge/version-for-repo/gnuguix/kubo.svg)](https://packages.guix.gnu.org/packages/kubo/) |
| _other_ | [See Repology for the full list](https://repology.org/project/kubo/versions) | |

~~Snap~~ no longer supported ([#8688](https://github.com/ipfs/kubo/issues/8688))

#### macOS

| Manager | Install | Version |
|---------|---------|---------|
| Homebrew | `brew install ipfs` | [![Homebrew](https://repology.org/badge/version-for-repo/homebrew/kubo.svg)](https://formulae.brew.sh/formula/ipfs) |
| MacPorts | `sudo port install ipfs` | [![MacPorts](https://repology.org/badge/version-for-repo/macports/kubo.svg)](https://ports.macports.org/port/ipfs/) |
| Nix | `nix-env -i kubo` | [![nixpkgs unstable](https://repology.org/badge/version-for-repo/nix_unstable/kubo.svg)](https://search.nixos.org/packages?query=kubo) |
| _other_ | [See Repology for the full list](https://repology.org/project/kubo/versions) | |

#### Windows

| Manager | Install | Version |
|---------|---------|---------|
| Scoop | `scoop install kubo` | [![Scoop](https://repology.org/badge/version-for-repo/scoop/kubo.svg)](https://scoop.sh/#/apps?q=kubo) |
| _other_ | [See Repology for the full list](https://repology.org/project/kubo/versions) | |

~~Chocolatey~~ no longer supported ([#9341](https://github.com/ipfs/kubo/issues/9341))

## Documentation

| Topic | Description |
|-------|-------------|
| [Configuration](docs/config.md) | All config options reference |
| [Environment variables](docs/environment-variables.md) | Runtime settings via env vars |
| [Experimental features](docs/experimental-features.md) | Opt-in features in development |
| [HTTP Gateway](docs/gateway.md) | Path, subdomain, and trustless gateway setup |
| [HTTP RPC clients](docs/http-rpc-clients.md) | Client libraries for Go, JS |
| [Delegated routing](docs/delegated-routing.md) | Multi-router and HTTP routing |
| [Metrics & monitoring](docs/metrics.md) | Prometheus metrics |
| [Content blocking](docs/content-blocking.md) | Denylist for public nodes |
| [Customizing](docs/customizing.md) | Unsure if use Plugins, Boxo, or fork? |
| [Debug guide](docs/debug-guide.md) | CPU profiles, memory analysis, tracing |
| [Changelogs](docs/changelogs/) | Release notes for each version |
| [All documentation](https://github.com/ipfs/kubo/tree/master/docs) | Full list of docs |

## Development

See the [Developer Guide](docs/developer-guide.md) for build instructions, testing, and contribution workflow.

## Getting Help

- [IPFS Forum](https://discuss.ipfs.tech) - community support, questions, and discussion
- [Community](https://docs.ipfs.tech/community/) - chat, events, and working groups
- [GitHub Issues](https://github.com/ipfs/kubo/issues) - bug reports for Kubo specifically
- [IPFS Docs Issues](https://github.com/ipfs/ipfs-docs/issues) - documentation issues

## Security Issues

See [`SECURITY.md`](SECURITY.md).

## Contributing

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

We welcome contributions. See [CONTRIBUTING.md](CONTRIBUTING.md) and the [Developer Guide](docs/developer-guide.md).

This repository follows the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

## Maintainer Info

<a href="https://ipshipyard.com/"><img align="right" src="https://github.com/user-attachments/assets/39ed3504-bb71-47f6-9bf8-cb9a1698f272" /></a>

> [!NOTE]
> Kubo is maintained by the [Shipyard](https://ipshipyard.com/) team.
>
> [Release Process](https://ipshipyard.notion.site/Kubo-Release-Process-6dba4f5755c9458ab5685eeb28173778)

## License

Dual-licensed under Apache 2.0 and MIT:

- [LICENSE-APACHE](LICENSE-APACHE)
- [LICENSE-MIT](LICENSE-MIT)
