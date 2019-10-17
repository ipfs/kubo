# Using go-ipfs as a Library. Learn to spawn a node and add a file to the IPFS network

> This tutorial is the sister of the [js-ipfs IPFS 101 tutorial](https://github.com/ipfs/js-ipfs/tree/master/examples/ipfs-101).

By the end of this Tutorial, you will learn how to:

- Spawn an IPFS node that runs in process (no separate daemon process)
- How to create an IPFS Repo
- How to add files & directories to IPFS
- How to retrieve those files and directories using cat and get
- How to connect to other nodes in the Network
- How to retrieve a file that only exists on the Network
- The difference between a node in DHT Client mode and Full DHT mode.

All of this using only golang!

You will need:
- golang installed on your machine. See how at https://golang.org/doc/install
- git installed on your machine so that go can download the repo and the necessary dependencies. See how at https://git-scm.com/downloads

## Getting started

Download go-ipfs and jump into the example folder

```
> go get -u github.com/ipfs/go-ipfs
cd $GOPATH/src/github.com/ipfs/go-ipfs/docs/examples/go-ipfs-as-a-library
```
