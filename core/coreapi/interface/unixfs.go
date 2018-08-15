package iface

import (
	"context"
	"io"

	ipld "gx/ipfs/QmaA8GkXUYinkkndvg7T6Tx7gYXemhxjaxLisEPes7Rf1P/go-ipld-format"
)

// UnixfsAPI is the basic interface to immutable files in IPFS
type UnixfsAPI interface {
	// Add imports the data from the reader into merkledag file
	Add(context.Context, io.Reader) (ResolvedPath, error)

	// Cat returns a reader for the file
	Cat(context.Context, Path) (Reader, error)

	// Ls returns the list of links in a directory
	Ls(context.Context, Path) ([]*ipld.Link, error)
}
