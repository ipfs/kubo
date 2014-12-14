# go-ipfs/cmd/ipfs

This is the ipfs commandline tool. For now, it's the main entry point to using IPFS. Use it.

```
> go build
> go install
> ipfs
ipfs - global versioned p2p merkledag file system

Basic commands:

    init          Initialize ipfs local configuration
    add <path>    Add an object to ipfs
    cat <ref>     Show ipfs object data
    ls <ref>      List links from an object

Tool commands:

    config        Manage configuration
    update        Download and apply go-ipfs updates
    version       Show ipfs version information
    commands      List all available commands

Advanced Commands:

    mount         Mount an ipfs read-only mountpoint
    serve         Serve an interface to ipfs
    diag          Print diagnostics

Plumbing commands:

    block         Interact with raw blocks in the datastore
    object        Interact with raw dag nodes


Use "ipfs help <command>" for more information about a command.
```
