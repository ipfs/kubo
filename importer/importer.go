package importer

import (
	"fmt"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"io"
	"io/ioutil"
)

var BlockSizeLimit = int64(1048576) // 1 MB
var SizeLimitExceeded = fmt.Errorf("object size limit exceeded")

// todo: incremental construction with an ipfs node. dumping constructed
// objects into the datastore, to avoid buffering all in memory

// size required for block construction
func NewDagFromReader(r io.Reader, size int64) (*dag.Node, error) {
	// todo: block-splitting based on rabin fingerprinting
	// todo: block-splitting with user-defined function
	// todo: block-splitting at all. :P

	// totally just trusts the reported size. fix later.
	if size > BlockSizeLimit { // 1 MB limit for now.
		return nil, SizeLimitExceeded
	}

	// we're doing it live!
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if int64(len(buf)) > BlockSizeLimit {
		return nil, SizeLimitExceeded // lying punk.
	}

	root := &dag.Node{Data: buf}
	// no children for now because not block splitting yet
	return root, nil
}
