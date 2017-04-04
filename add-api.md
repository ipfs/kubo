A clear picture of the intended adder API is blocking some of the
later commits in #1136.  This issue floats my first pass at an adder
API that I think strikes a good balance between power and simplicity
(mostly by offloading the complicated bits to a per-DAG-node
callback).

# Where it lives

I'd suggest moving `core/coreunix` to `core/unixfs` to match
`unixfs/` and `shell/unixfs/`.

# High-level UI (shell/unixfs/)

    // PreNodeCallback is called before each DAG node is created.  The
    // arguments are:
    //
    //   path: The path from the add root to the just-created node.
    //     This is empty for the root node.  For large files and
    //     directories that are chunked and laid-out, each chunk and
    //     fanout node will have the same path argument.
    //   file: A File reference for the file used to create the new
    //     nodes.  Don't seek this (which could throw off the chunker),
    //     but you can use it to extract metadata about the visited
    //     file including its name, permissions, etc.  This will be
    //     nil for io.Reader-based adders.
    //
    // The returned values are:
    //
    //   ignore: True if we should skip this path.
    //   err: Any errors serious enough to abort the addition.
    type PreNodeCallback func(
        path *path.Path,
        file *os.File
        ) (ignore bool, err error)

    // PostNodeCallback is called after each DAG node is created.  The
    // arguments are:
    //
    //   nodeIn: The just-created node.
    //   path: The path from the add root to the just-created node.
    //     This is empty for the root node.  For large files and
    //     directories that are chunked and laid-out, each chunk and
    //     fanout node will have the same path argument.
    //   file: A File reference for the file used to create nodeIn.
    //     Don't seek this (which could throw off the chunker), but
    //     you can use it to extract metadata about the visited file
    //     including its name, permissions, etc.  This will be nil for
    //     io.Reader-based adders.
    //   top: Whether or not nodeIn is the tip of a layout DAG or an
    //     unchunked object.  This allows you to distinguish those
    //     nodes (which are referenced from a link with a user-visible
    //     name)
    //
    // The returned values are:
    //
    //   nodeOut: The node to insert into the constructed DAG.  Return
    //     nodeIn to use the just-created node without changes or nil
    //     to drop the just-created node.  You're also free to return
    //     another node of your choosing (e.g. a new node wrapping the
    //     just-created node or a completely independent node).
    //   err: Any errors serious enough to abort the addition.
    type PostNodeCallback func(
        nodeIn *dag.Node,
        path *path.Path,
        file *os.File,
        top bool
        ) (nodeOut *dag.Node, err error)

    // Add adds a single file from an io.Reader.  The arguments are:
    //
    //   ctx: A Context for cancelling or timing out a recursive
    //     addition.
    //   node: And IPFS node for storing newly-created DAG nodes.
    //   reader: A Reader from which the file contents are read.
    //   preNodeCallBack: An optional hook for pre-DAG-node checks
    //     (e.g. ignoring boring paths).  Set to nil if you don't need
    //     it.
    //   postNodeCallBack: An optional hook for post-DAG-node
    //     processing (e.g. altering or wrapping the newly-created
    //     nodes).  Set to nil if you don't need it.
    //
    // The returned values are:
    //
    //   root: The root of the just-added DAG.  Even though we're only
    //     adding a single file, layout and chunking choices may lead
    //     to the creation of several Merkle objects.
    //   err: Any errors serious enough to abort the addition.
    Add(
        ctx context.Context,
        node *core.IpfsNode,
        reader io.Reader,
        preNodeCallBack *PreNodeCallback,
        postNodeCallback *PostNodeCallback
        ) (root *dag.Node, err error)

    // AddFile recursively adds files from a File type, which can
    // point to either a directory or a file.  The signature matches
    // Add, except that the 'reader' io.Reader is replaced by the
    // 'file' *os.File.
    AddFile(
        ctx context.Context,
        node *core.IpfsNode,
        file *os.File,
        preNodeCallBack *PreNodeCallback,
        postNodeCallback *PostNodeCallback
        ) (root *dag.Node, err error)

Single file additions can use a [Reader][], but filesystem-based
additions may find the recursive, [*File][File]-based AddFile more
convenient, because:

1. It handles directory recursion internally, and
2. It allows access to local metadata (access mode, ownership, …) and
   root-relative filenames for use by the pre- and post-node
   callbacks.

We need a way to get information about progress of a running addition
back to other goroutines.  Choices for this include [the channel
messages proposed in #1121][channel] or additional arguments to [a
per-chunk callback like that proposed in #1274][callback].  The main
difference between a callback and a channel is whether or not we want
synchronous collaboration between the adder and the hook.  Since I
think we want the option for synchronous collaboration
(e.g. optionally inserting a metadata node on top of each file node).
For situations where asynchronous communication makes more sense, the
user can provide a synchronous callback that just pushes a message to
a channel (so the callback-based API supports the channel-based
workflow).

Actions like wrapping an added file in another Merkle object to hold a
filename is left to the caller and the callback API.

# Low-level UI (core/unixfs/)

These should look just like the high-level API, except instead of
passing in an IpfsNode and using that node's default DAG service,
layout, and splitter, we pass each of those in explicitly:

    Add(
        ctx context.Context,
        dagService dag.DAGService,
        layout _layout.Layout,
        splitter chunk.BlockSplitter,
        reader io.Reader,
        preNodeCallBack *PreNodeCallback,
        postNodeCallback *PostNodeCallback
        ) (root *dag.Node, error)
    AddFile(
        ctx context.Context,
        dagService dag.DAGService,
        layout _layout.Layout,
        splitter chunk.BlockSplitter,
        file *os.File,
        preNodeCallBack *PreNodeCallback,
        postNodeCallback *PostNodeCallback
        ) (root *dag.Node, err error)

We don't currently have a public `Layout` interface, but I think we
should add one so folks can easily plug in alternative layout
implementations.

I'm not familiar enough with Go at the moment to know which arguments
are best passed by reference and which should be passed by value.  If
I was writing this in C, everything except the Boolean `top` would be
passed by reference, but I'll have to read around to figure out what's
idiomatic in Go.

[File]: https://golang.org/pkg/os/#File
[Reader]: https://golang.org/pkg/io/#Reader
[channel]: https://github.com/ipfs/go-ipfs/issues/1121#issuecomment-104073727
[callback]: https://github.com/ipfs/go-ipfs/pull/1274
