package importer

import (
	"fmt"
	"io"
	"os"

	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/pin"
	ft "github.com/jbenet/go-ipfs/unixfs"
	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("importer")

// BlockSizeLimit specifies the maximum size an imported block can have.
var BlockSizeLimit = int64(1048576) // 1 MB

// ErrSizeLimitExceeded signals that a block is larger than BlockSizeLimit.
var ErrSizeLimitExceeded = fmt.Errorf("object size limit exceeded")

// todo: incremental construction with an ipfs node. dumping constructed
// objects into the datastore, to avoid buffering all in memory

// NewDagFromReader constructs a Merkle DAG from the given io.Reader.
// size required for block construction.
func NewDagFromReader(r io.Reader) (*dag.Node, error) {
	return NewDagFromReaderWithSplitter(r, chunk.DefaultSplitter)
}

func NewDagFromReaderWithSplitter(r io.Reader, spl chunk.BlockSplitter) (*dag.Node, error) {
	blkChan := spl.Split(r)
	first := <-blkChan
	root := &dag.Node{}

	mbf := new(ft.MultiBlock)
	for blk := range blkChan {
		mbf.AddBlockSize(uint64(len(blk)))
		child := &dag.Node{Data: ft.WrapData(blk)}
		err := root.AddNodeLink("", child)
		if err != nil {
			return nil, err
		}
	}

	mbf.Data = first
	data, err := mbf.GetBytes()
	if err != nil {
		return nil, err
	}

	root.Data = data
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

	return NewDagFromReader(f)
}

func BuildDagFromFile(fpath string, ds dag.DAGService, mp pin.ManualPinner) (*dag.Node, error) {
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

	return BuildDagFromReader(f, ds, mp, chunk.DefaultSplitter)
}

func BuildDagFromReader(r io.Reader, ds dag.DAGService, mp pin.ManualPinner, spl chunk.BlockSplitter) (*dag.Node, error) {
	blkChan := spl.Split(r)

	// grab first block, it will go in the index MultiBlock (faster io)
	first := <-blkChan
	root := &dag.Node{}

	mbf := new(ft.MultiBlock)
	for blk := range blkChan {
		// Store the block size in the root node
		mbf.AddBlockSize(uint64(len(blk)))
		node := &dag.Node{Data: ft.WrapData(blk)}
		nk, err := ds.Add(node)
		if err != nil {
			return nil, err
		}

		if mp != nil {
			mp.PinWithMode(nk, pin.Indirect)
		}

		// Add a link to this node without storing a reference to the memory
		err = root.AddNodeLinkClean("", node)
		if err != nil {
			return nil, err
		}
	}

	// Generate the root node data
	mbf.Data = first
	data, err := mbf.GetBytes()
	if err != nil {
		return nil, err
	}
	root.Data = data

	// Add root node to the dagservice
	rootk, err := ds.Add(root)
	if err != nil {
		return nil, err
	}
	if mp != nil {
		mp.PinWithMode(rootk, pin.Recursive)
	}

	return root, nil
}
