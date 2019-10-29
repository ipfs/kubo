# Use go-ipfs as a library to spawn a node and add a file

> This tutorial is the sibling of the [js-ipfs IPFS 101 tutorial](https://github.com/ipfs/js-ipfs/tree/master/examples/ipfs-101).

By the end of this tutorial, you will learn how to:

- Spawn an IPFS node that runs in process (no separate daemon process)
- Create an IPFS repo
- Add files and directories to IPFS
- Retrieve those files and directories using ``cat`` and ``get``
- Connect to other nodes in the network
- Retrieve a file that only exists on the network
- The difference between a node in DHT client mode and full DHT mode

All of this using only golang!

In order to complete this tutorial, you will need:
- golang installed on your machine. See how at https://golang.org/doc/install
- git installed on your machine (so that go can download the repo and the necessary dependencies). See how at https://git-scm.com/downloads
- IPFS Desktop (for convenience) installed and running on your machine. See how at https://github.com/ipfs-shipyard/ipfs-desktop#ipfs-desktop


**Disclaimer**: The example code is quite large (more than 300 lines of code) and it has been a great way to understand the scope of the [go-ipfs Core API](https://godoc.org/github.com/ipfs/interface-go-ipfs-core), and how it can be improved to further the user experience. You can expect to be able to come back to this example in the future and see how the number of lines of code have decreased and how the example have become simpler, making other go-ipfs programs simpler as well.

## Getting started

**Note:** Make sure you have [![](https://img.shields.io/badge/go-%3E%3D1.13.0-blue.svg?style=flat-square)](https://golang.org/dl/) installed.

Download go-ipfs and jump into the example folder:

```
> go get -u github.com/ipfs/go-ipfs
cd $GOPATH/src/github.com/ipfs/go-ipfs/docs/examples/go-ipfs-as-a-library
```

## Running the example as-is

To run the example, simply do:

```
> go run main.go
```

You should see the following as output:

```
-- Getting an IPFS node running --
Spawning node on a temporary repo
IPFS node is running

-- Adding and getting back files & directories --
Added file to IPFS with CID /ipfs/QmV9tSDx9UiPeWExXEeH6aoDvmihvx6jD5eLb4jbTaKGps
Added directory to IPFS with CID /ipfs/QmdQdu1fkaAUokmkfpWrmPHK78F9Eo9K2nnuWuizUjmhyn
Got file back from IPFS (IPFS path: /ipfs/QmV9tSDx9UiPeWExXEeH6aoDvmihvx6jD5eLb4jbTaKGps) and wrote it to ./example-folder/QmV9tSDx9UiPeWExXEeH6aoDvmihvx6jD5eLb4jbTaKGps
Got directory back from IPFS (IPFS path: /ipfs/QmdQdu1fkaAUokmkfpWrmPHK78F9Eo9K2nnuWuizUjmhyn) and wrote it to ./example-folder/QmdQdu1fkaAUokmkfpWrmPHK78F9Eo9K2nnuWuizUjmhyn

-- Going to connect to a few nodes in the Network as bootstrappers --
Fetching a file from the network with CID QmUaoioqU7bxezBQZkUcgcSyokatMY71sxsALxQmRRrHrj
Wrote the file to ./example-folder/QmUaoioqU7bxezBQZkUcgcSyokatMY71sxsALxQmRRrHrj

All done! You just finalized your first tutorial on how to use go-ipfs as a library
```

## Understanding the example

In this example, we add a file and a directory with files; we get them back from IPFS; and then we use the IPFS network to fetch a file that we didn't have in our machines before.

Each section below has links to lines of code in the file [main.go](./main.go). The code itself will have comments explaining what is happening for you.

### The `func main() {}`

The [main function](./main.go#L202-L331) is where the magic starts, and it is the best place to follow the path of what is happening in the tutorial.

### Part 1: Getting an IPFS node running

To get [get a node running](./main.go#L218-L223) as an [ephemeral node](./main.go#L114-L128) (that will cease to exist when the run ends), you will need to:

- [Prepare and set up the plugins](./main.go#L30-L47)
- [Create an IPFS repo](./main.go#L49-L68)
- [Construct the IPFS node instance itself](./main.go#L72-L96)

As soon as you construct the IPFS node instance, the node will be running.

### Part 2: Adding a file and a directory to IPFS

- [Prepare the file to be added to IPFS](./main.go#L166-L184)
- [Add the file to IPFS](./main.go#L240-L243)
- [Prepare the directory to be added to IPFS](./main.go#L186-L198)
- [Add the directory to IPFS](./main.go#L252-L255)

### Part 3: Getting the file and directory you added back

- [Get the file back](./main.go#L265-L268)
- [Write the file to your local filesystem](./main.go#L270-L273)
- [Get the directory back](./main.go#L277-L280)
- [Write the directory to your local filesystem](./main.go#L282-L285)

### Part 4: Getting a file from the IPFS network

- [Connect to nodes in the network](./main.go#L293-L310)
- [Get the file from the network](./main.go#L318-L321)
- [Write the file to your local filesystem](./main.go#L323-L326)

### Bonus: Spawn a daemon on your existing IPFS repo (on the default path ~/.ipfs)

As a bonus, you can also find lines that show you how to spawn a node over your default path (~/.ipfs) in case you had already started a node there before. To try it:

- [Comment these lines](./main.go#L219-L223)
- [Uncomment these lines](./main.go#L209-L216)

## Voilá! You are now a go-ipfs hacker

You've learned how to spawn a go-ipfs node using the go-ipfs core API. There are many more [methods to experiment next](https://godoc.org/github.com/ipfs/interface-go-ipfs-core). Happy hacking!
