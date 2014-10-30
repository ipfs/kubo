package importer

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/pin"
	ft "github.com/jbenet/go-ipfs/unixfs"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
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

func NewDagFromFileWServer(fpath string, dserv dag.DAGService, p pin.Pinner) (*dag.Node, error) {
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

	return NewDagFromReaderWServer(f, dserv, p)
}

func NewDagFromReaderWServer(r io.Reader, dserv dag.DAGService, p pin.Pinner) (*dag.Node, error) {
	dw := uio.NewDagWriter(dserv, chunk.DefaultSplitter)

	mp, ok := p.(pin.ManualPinner)
	if !ok {
		return nil, errors.New("Needed to be passed a manual pinner!")
	}
	dw.Pinner = mp
	_, err := io.Copy(dw, r)
	if err != nil {
		return nil, err
	}
	err = dw.Close()
	if err != nil {
		return nil, err
	}
	return dw.GetNode(), nil
}
