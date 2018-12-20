https://github.com/ipfs/go-ipfs/blob/master/core/commands/add.go#L208-L213

The goal of this document is to capture the code flow for adding a file (see the `coreapi` package) using the IPFS CLI, in the process exploring some datastructures and packages like `ipld.Node` (aka `dagnode`, `FSNode`, `MFS`, etc.

# `UnixfsAPI.Add()`
*Entrypoint into the `Unixfs` package*

https://github.com/ipfs/go-ipfs/blob/master/core/coreapi/unixfs.go#L78-L86

The `UnixfsAPI.Add()` acts on the input data or files, to build a _merkledag_ node (in essence it is the entire tree represented by the root node) and adds it to the _blockstore_.
Within the function, a new `Adder` is created with the configured `Blockstore` and __DAG service__`. 


# `Adder.AddAllAndPin(files.File)`
*Entrypoint*

https://github.com/ipfs/go-ipfs/blob/master/core/coreunix/add.go#L427-L431

The more interesting stuff happens in the `Adder.AddAllAndPin(files)` function. 

https://github.com/ipfs/go-ipfs/blob/master/core/coreunix/add.go#L467-L476

Let's focus on the simple case of a single file, handled by `Adder.addFileNode(path, file)` which redirects the case to `Adder.addFile(pathm files.File)`. 

## `Adder.addFile(path, files.File)`
*Create the _DAG_ and add to `MFS`*

The `addFile(file)` method is responsible taking the data and converting it into a __DAG__ tree, followed by adding the root of the DAG tree in to the `MFS`.

https://github.com/ipfs/go-ipfs/blob/master/core/coreunix/add.go#L508-L521

There are two main methods to focus on -

### `Adder.add(io.Reader)`
*Create and return the **root** __DAG__ node*

https://github.com/ipfs/go-ipfs/blob/master/core/coreunix/add.go#L115-L137

This method converts the input _data_ (`io.Reader`) to a `DAG` tree. This is done by splitting the data into _chunks_ using the `Chunker` **(HELP: elaborate on chunker types ?)** and organizing them in a `DAG` (with a *trickle* or *balanced* layout). The method returns the **root** of the __DAG__, formatted as an `ipld.Node`.

### `Adder.addNode(ipld.Node, path)`
*Add **root** __DAG__ node to the `MFS`*

https://github.com/ipfs/go-ipfs/blob/master/core/coreunix/add.go#L365-L399

Now that we have the **root** node of the `DAG`, this needs to be added to the `MFS` file system. 
The `MFS` **root** is first fetched (or created, if doesn't already exist) by invoking `mfsRoot()`. 
Assuming the directory structure already exists in the MFS file system, (if it doesn't exist it will be created using `mfs.Mkdir()` function by passing in the `MFS` **root**), the **root** __DAG__ node is added to the `MFS` File system within the `mfs.PutNode()` function.

#### `[MFS] PutNode(mfs.Root, path, ipld.Node)`
*Insert node at path into given `MFS`*

https://github.com/ipfs/go-mfs/blob/master/ops.go#L101-L113

In this the path is used to determine the `MFS` `Directory`, which is first lookup up in the `MFS` using `lookupDir()` function. This is followed by adding the **root** __DAG__ node (`ipld.Node`) in to the found `Directory` using `directory.AddChild()` method.

#### - `directory.AddChild(filename, ipld.Node)`
*Add **root** __DAG__ node , as filename, under this directory*

https://github.com/ipfs/go-mfs/blob/master/dir.go#L381-L402

Within this method the node is added to the __DAG service__ of the `Directory` object using the `dserv.Add()` method [HELP NEEDED].
This is subsequently followed by adding the **root** __DAG__ node by creating a `directory.child{}` object with the given name, within in the `directory.addUnixFSChild(directory.child{name, ipld.Node})` method.

#### -- `directory.addUnixFSChild(child)`
*Switch to HAMT (if configured) and add child to inner UnixFS Directory*

https://github.com/ipfs/go-mfs/blob/master/dir.go#L406-L425

Here the transition of the UnixFS directory to __HAMT__ implemetation is done, if configured, wherein if the directory is of type `BasicDirectory`, it is converted to a __HAMT__ implementation.
The node is then added as a child to the inner `UnixFS` directory using the `directory.AddChild()` method.
Note: This is not to be confused with the `directory.AddChild(filename, ipld.Node)`, as this operates on the inner `UnixFS` `Directory` object only.

#### --- (inner)`Directory.AddChild(ctx, name, ipld.Node)`

This method vastly differs based on the inner `Directory` implementation (Basic vs HAMT). Let's focus on the `BasicDirectory` implementation to keep things simple.

https://github.com/ipfs/go-unixfs/blob/master/io/directory.go#L142-L147

> IMPORTANT
> It should be noted that the inner `Directory` of the `UnixFS` package, encasulates a node object of type `ProtoNode`, which is a different format as compared to the `ipld.Node` we have been working on throughout this document.

This method first attempts to remove any old links (`ProtoNode.RemoveNodeLink(name)`) to the `ProtoNode` prior to adding a link to the newly added `ipld/Node`, using `ProtoNode.AddNodeLink(name, ipld.Node)`.

https://github.com/ipfs/go-merkledag/blob/master/node.go#L99-L112

The `AddNodeLink()` method is where an `ipld.Link` is created with the `ipld.Node`'s `CID` and size in the `ipld.MakeLink(ipld.Node)` method, and is then appended to the `ProtoNode`'s links in the `ProtoNode.AddRawLink(name)` method.

---

## `adder.Finalize()`
*Fetch and return the __DAG__ **root** from the `MFS` and `UnixFS` directory*

https://github.com/ipfs/go-ipfs/blob/master/core/coreunix/add.go#L199-L244

The whole process ends with `adder.Finalize()` which returns the `ipld.Node` from the `UnixFS` `Directory`.
**(HELP: Do we need to elaborate ?)**