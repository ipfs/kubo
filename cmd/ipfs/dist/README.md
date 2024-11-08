# ipfs command line tool

This is a [command line tool for interacting with Kubo](https://docs.ipfs.tech/install/command-line/),
an [IPFS](https://ipfs.tech) implementation. It contains a full IPFS node.

## Install

To install it, move the binary somewhere in your `$PATH`:

```sh
sudo mv ipfs /usr/local/bin/ipfs
```

Or run `sudo ./install.sh` which does this for you.

## Usage

First, you must initialize your local ipfs node:

```sh
ipfs init
```

This will give you directions to get started with ipfs.
You can always get help with:

```sh
ipfs --help
```
