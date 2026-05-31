# Use Kubo (go-ipfs) as a library to spawn a node and add a file

> Note: if you are trying to customize or extend Kubo, you should read the [Customizing Kubo](../../customizing.md) doc

By the end of this tutorial, you will learn how to:

- Spawn an IPFS node that runs in process (no separate daemon process)
- Create an IPFS repo
- Add files and directories to IPFS
- Retrieve those files and directories using ``cat`` and ``get``
- Connect to other nodes in the network
- Retrieve a file that only exists on the network
- Publish and receive a PubSub message between two in-process nodes
- The difference between a node in DHT client mode and full DHT mode

All of this using only golang!

In order to complete this tutorial, you will need:
- golang installed on your machine. See how at https://golang.org/doc/install
- git installed on your machine (so that go can download the repo and the necessary dependencies). See how at https://git-scm.com/downloads
- IPFS Desktop (for convenience) installed and running on your machine. See how at https://github.com/ipfs-shipyard/ipfs-desktop#ipfs-desktop


**Disclaimer**: The example code is quite large (more than 300 lines of code) and it has been a great way to understand the scope of the [go-ipfs Core API](https://godoc.org/github.com/ipfs/interface-go-ipfs-core), and how it can be improved to further the user experience. You can expect to be able to come back to this example in the future and see how the number of lines of code have decreased and how the example have become simpler, making other go-ipfs programs simpler as well.

## Getting started

**Note:** Make sure you have [![](https://img.shields.io/badge/go-%3E%3D1.13.0-blue.svg?style=flat-square)](https://golang.org/dl/) installed.

Download Kubo and jump into the example folder:

```console
$ git clone https://github.com/ipfs/kubo.git
$ cd kubo/docs/examples/kubo-as-a-library
```

## Running the example as-is

To run the example, simply do:

```console
$ go run main.go
```

You should see output similar to:

```
-- Getting an IPFS node running --
Spawning Kubo node on a temporary repo
IPFS node is running
Connecting to peer...
Connected to peer
Added file to peer with CID /ipfs/...

-- Adding and getting back files & directories --
Added file to IPFS with CID /ipfs/...
Added directory to IPFS with CID /ipfs/...
output folder: /tmp/example...
got file back from IPFS (IPFS path: /ipfs/...) and wrote it to /tmp/example...
Got directory back from IPFS (IPFS path: /ipfs/...) and wrote it to /tmp/example...

-- Fetching content from nodeA via bitswap --
Fetching a file from the network with CID ...
Wrote the file to /tmp/example...

-- Publishing and subscribing with IPFS PubSub --
Received pubsub message from 12D3KooW... on topic "kubo-as-a-library": hello from kubo pubsub

All done! You just finalized your first tutorial on how to use Kubo as a library
```

## Understanding the example

In this example, we add a file and a directory with files; we get them back from IPFS; use another in-process node to fetch a file over bitswap; and publish a message between nodes with IPFS PubSub.

Each section below links to the relevant code in [main.go](./main.go). The code itself will have comments explaining what is happening for you.

### The `func main() {}`

The [main function](./main.go) is where the magic starts, and it is the best place to follow the path of what is happening in the tutorial.

### Part 1: Getting an IPFS node running

To get a node running as an [ephemeral node](./main.go) (that will cease to exist when the run ends), you will need to:

- [Prepare and set up the plugins](./main.go)
- [Create an IPFS repo](./main.go)
- [Construct the IPFS node instance itself](./main.go)

As soon as you construct the IPFS node instance, the node will be running.

### Part 2: Adding a file and a directory to IPFS

- [Prepare the file to be added to IPFS](./main.go)
- [Add the file to IPFS](./main.go)
- [Prepare the directory to be added to IPFS](./main.go)
- [Add the directory to IPFS](./main.go)

### Part 3: Getting the file and directory you added back

- [Get the file back](./main.go)
- [Write the file to your local filesystem](./main.go)
- [Get the directory back](./main.go)
- [Write the directory to your local filesystem](./main.go)

### Part 4: Getting a file from the IPFS network

- [Connect to nodes in the network](./main.go)
- [Get the file from the network](./main.go)
- [Write the file to your local filesystem](./main.go)

### Part 5: Publishing and subscribing with IPFS PubSub

This section uses Kubo's built-in PubSub for a minimal local Core API demonstration. For production custom PubSub applications, prefer [go-libp2p-pubsub](https://github.com/libp2p/go-libp2p-pubsub) directly.

- Enable PubSub in the node configuration and build options
- Subscribe one node to a topic with the Core API
- Publish a message from another connected node
- Wait until the subscriber receives the expected message

### Bonus: Spawn a daemon on your existing IPFS repo (on the default path ~/.ipfs)

As a bonus, you can also find lines that show you how to spawn a node over your default path (~/.ipfs) in case you had already started a node there before. To try it:

- Comment the temporary repo lines in [main.go](./main.go)
- Uncomment the default repo lines in [main.go](./main.go)

## Voilá! You are now a Kubo hacker

You've learned how to spawn a Kubo node using the Core API. There are many more [methods to experiment next](https://godoc.org/github.com/ipfs/interface-go-ipfs-core). Happy hacking!
