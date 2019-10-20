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

You complete this tutorial, You will need:
- golang installed on your machine. See how at https://golang.org/doc/install
- git installed on your machine so that go can download the repo and the necessary dependencies. See how at https://git-scm.com/downloads
- Have IPFS Desktop (for convinience) installed and running on your machine. See how at https://github.com/ipfs-shipyard/ipfs-desktop#ipfs-desktop


**Disclaimer**: The example code is quite large (over 300 lines of code) and it has been a great way to understand the scope of the [go-ipfs Core API](https://godoc.org/github.com/ipfs/interface-go-ipfs-core) and how it can be improved to further the user experience. You can expect to come back at this example in the future and see how the number of lines of code decreases and how the example becomes simpler, making other go-ipfs programs simpler as well.

## Getting started

Download go-ipfs and jump into the example folder

```
> go get -u github.com/ipfs/go-ipfs
cd $GOPATH/src/github.com/ipfs/go-ipfs/docs/examples/go-ipfs-as-a-library
```

## Running the example as is

To run the example, simply do:

```
> go run main.go
```

You should see as output:

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

In this example, we add a file, a directory with files, we get them back from IPFS and then we use the IPFS network to fetch a file that we didn't have in our machines yet.

### The `func main() {}`

### Part I: Getting an IPFS node Running

### Part II: Adding a file and a directory to IPFS

### Part III: Getting the file and directory you added back

### Part IV: Getting a file from the IPFS Network

### Bonus: Spawn a daemon on your existing IPFS Repo (on the default path ~/.ipfs)
