package importer

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	dag "github.com/jbenet/go-ipfs/merkledag"
)

// BlockSizeLimit specifies the maximum size an imported block can have.
var BlockSizeLimit = int64(1048576) // 1 MB

// ErrSizeLimitExceeded signals that a block is larger than BlockSizeLimit.
var ErrSizeLimitExceeded = fmt.Errorf("object size limit exceeded")

// todo: incremental construction with an ipfs node. dumping constructed
// objects into the datastore, to avoid buffering all in memory

// NewDagFromReader constructs a Merkle DAG from the given io.Reader.
// size required for block construction.
func NewDagFromReader(r io.Reader, size int64) (*dag.Node, error) {
	// todo: block-splitting based on rabin fingerprinting
	// todo: block-splitting with user-defined function
	// todo: block-splitting at all. :P
	// todo: write mote todos

	// totally just trusts the reported size. fix later.
	if size > BlockSizeLimit { // 1 MB limit for now.
		return nil, ErrSizeLimitExceeded
	}

	// Ensure that we dont get stuck reading way too much data
	r = io.LimitReader(r, BlockSizeLimit)

	// we're doing it live!
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if int64(len(buf)) > BlockSizeLimit {
		return nil, ErrSizeLimitExceeded // lying punk.
	}

	root := &dag.Node{Data: buf}
	// no children for now because not block splitting yet
	return root, nil
}

// NewDagFromFile constructs a Merkle DAG from the file at given path.
func NewDagFromFile(fpath string) (*dag.Node, error) {
	stat, err := os.Stat(fpath)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("`%s` is a directory", fpath)
	}

	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return NewDagFromReader(f, stat.Size())
}
