# `core/coreunix`'s `Add` → `shell/unixfs`'s `AddFromReader`

We've added an `AddFromReader` function to `shell/unixfs`.  The old
`Add` from `core/coreunix` is deprecated and will be removed in
version 0.4.0. To update your existing code, change usage like:

    keyString, err := coreunix.Add(ipfsNode, reader)
    if err != nil {
        return err
    }

to

    fileNode, err := unixfs.AddFromReader(ipfsNode, reader)
    if err != nil {
        return err
    }
    key, err := fileNode.Key()
    if err != nil {
        return err
    }
    keyString := key.String()

That's a bit more verbose if all you want is the stringified key, but
returning a `dag.Node` makes it easier to perform other operations
like adding the file node to a directory node.
