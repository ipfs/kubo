# IPFS : The `Add` command demystified

The goal of this document is to capture the code flow for adding a file (see the `coreapi` package) using the IPFS CLI, in the process exploring some datastructures and packages like `ipld.Node` (aka `dagnode`), `FSNode`, `MFS`, etc.

## Concepts
- [Files](https://github.com/ipfs/docs/issues/133)

--- 

**Try this yourself**
> 
> ```
> # Convert a file to the IPFS format.
> echo "Hello World" > new-file
> ipfs add new-file
> added QmWATWQ7fVPP2EFGu71UkfnqhYXDYH566qy47CnJDgvs8u new-file
> 12 B / 12 B [=========================================================] 100.00%
>
> # Add a file to the MFS.
> NEW_FILE_HASH=$(ipfs add new-file -Q)
> ipfs files cp /ipfs/$NEW_FILE_HASH /new-file
> 
> # Get information from the file in MFS.
> ipfs files stat /new-file
> # QmWATWQ7fVPP2EFGu71UkfnqhYXDYH566qy47CnJDgvs8u
> # Size: 12
> # CumulativeSize: 20
> # ChildBlocks: 0
> # Type: file
> 
> # Retrieve the contents.
> ipfs files read /new-file
> # Hello World
> ```

## Code Flow

**[`UnixfsAPI.Add()`](https://github.com/ipfs/go-ipfs/blob/v0.4.18/core/coreapi/unixfs.go#L31)** - *Entrypoint into the `Unixfs` package*

The `UnixfsAPI.Add()` acts on the input data or files, to build a _merkledag_ node (in essence it is the entire tree represented by the root node) and adds it to the _blockstore_.
Within the function, a new `Adder` is created with the configured `Blockstore` and __DAG service__`. 

- **[`adder.AddAllAndPin(files)`](https://github.com/ipfs/go-ipfs/blob/v0.4.18/core/coreunix/add.go#L403)** - *Entrypoint to the `Add` logic*
    encapsulates a lot of the underlying functionality that will be investigated in the following sections. 

    Our focus will be on the simplest case, a single file, handled by `Adder.addFile(file files.File)`. 

  - **[`adder.addFile(file files.File)`](https://github.com/ipfs/go-ipfs/blob/v0.4.18/core/coreunix/add.go#L450)** - *Create the _DAG_ and add to `MFS`*

      The `addFile(file)` method takes the data and converts it into a __DAG__ tree and adds the root of the tree into the `MFS`.

      https://github.com/ipfs/go-ipfs/blob/v0.4.18/core/coreunix/add.go#L508-L521

      There are two main methods to focus on -

      1. **[`adder.add(io.Reader)`](https://github.com/ipfs/go-ipfs/blob/v0.4.18/core/coreunix/add.go#L115)** - *Create and return the **root** __DAG__ node*

          This method converts the input data (`io.Reader`) to a __DAG__ tree, by splitting the data into _chunks_ using the `Chunker` and organizing them in to a __DAG__ (with a *trickle* or *balanced* layout. See [balanced](https://github.com/ipfs/go-unixfs/blob/6b769632e7eb8fe8f302e3f96bf5569232e7a3ee/importer/balanced/builder.go) for more info). 

          The method returns the **root** `ipld.Node` of the __DAG__.

      2. **[`adder.addNode(ipld.Node, path)`](https://github.com/ipfs/go-ipfs/blob/v0.4.18/core/coreunix/add.go#L366)** - *Add **root** __DAG__ node to the `MFS`*

          Now that we have the **root** node of the `DAG`, this needs to be added to the `MFS` file system. 
          Fetch (or create, if doesn't already exist) the `MFS` **root** using `mfsRoot()`. 

          > NOTE: The `MFS` **root** is an ephemeral root, created and destroyed solely for the `add` functionality.

          Assuming the directory already exists in the MFS file system, (if it doesn't exist it will be created using `mfs.Mkdir()`), the **root** __DAG__ node is added to the `MFS` File system using the `mfs.PutNode()` function.

          - **[MFS] [`PutNode(mfs.Root, path, ipld.Node)`](https://github.com/ipfs/go-mfs/blob/v0.1.18/ops.go#L86)** - *Insert node at path into given `MFS`*

              The `path` param is used to determine the `MFS Directory`, which is first looked up in the `MFS` using `lookupDir()` function. This is followed by adding the **root** __DAG__ node (`ipld.Node`) in to this `Directory` using `directory.AddChild()` method.

          - **[MFS] Add Child To `UnixFS`**
            - **[`directory.AddChild(filename, ipld.Node)`](https://github.com/ipfs/go-mfs/blob/v0.1.18/dir.go#L350)** - *Add **root** __DAG__ node under this directory*

                Within this method the node is added to the `Directory`'s __DAG service__ using the `dserv.Add()` method, followed by adding the **root** __DAG__ node with the given name, in the `directory.addUnixFSChild(directory.child{name, ipld.Node})` method.

            - **[MFS] [`directory.addUnixFSChild(child)`](https://github.com/ipfs/go-mfs/blob/v0.1.18/dir.go#L375)** - *Add child to inner UnixFS Directory*

                The node is then added as a child to the inner `UnixFS` directory using the `(BasicDirectory).AddChild()` method.

                > NOTE: This is not to be confused with the `directory.AddChild(filename, ipld.Node)`, as this operates on the `UnixFS` `BasicDirectory` object.

            - **[UnixFS] [`(BasicDirectory).AddChild(ctx, name, ipld.Node)`](https://github.com/ipfs/go-unixfs/blob/v1.1.16/io/directory.go#L137)** - *Add child to `BasicDirectory`*

                > IMPORTANT: It should be noted that the `BasicDirectory` object uses the `ProtoNode` type object which is an implementation of the `ipld.Node` interface, seen and used throughout this document. Ideally the `ipld.Node` should always be used, unless we need access to specific functions from `ProtoNode` (like `Copy()`) that are not available in the interface.

                This method first attempts to remove any old links (`ProtoNode.RemoveNodeLink(name)`) to the `ProtoNode` prior to adding a link to the newly added `ipld.Node`, using `ProtoNode.AddNodeLink(name, ipld.Node)`.

                - **[Merkledag] [`AddNodeLink()`](https://github.com/ipfs/go-merkledag/blob/v1.1.15/node.go#L99)**

                  The `AddNodeLink()` method is where an `ipld.Link` is created with the `ipld.Node`'s `CID` and size in the `ipld.MakeLink(ipld.Node)` method, and is then appended to the `ProtoNode`'s links in the `ProtoNode.AddRawLink(name)` method.

  - **[`adder.Finalize()`](https://github.com/ipfs/go-ipfs/blob/v0.4.18/core/coreunix/add.go#L200)** - *Fetch and return the __DAG__ **root** from the `MFS` and `UnixFS` directory*

      The `Finalize` method returns the `ipld.Node` from the `UnixFS` `Directory`.

  - **[`adder.PinRoot()`](https://github.com/ipfs/go-ipfs/blob/v0.4.18/core/coreunix/add.go#L171)** - *Pin all files under the `MFS` **root***

    The whole process ends with `PinRoot` recursively pinning all the files under the `MFS` **root**