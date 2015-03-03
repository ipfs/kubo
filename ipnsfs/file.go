package ipnsfs

import (
	"errors"
	"io"
	"os"
	"sync"

	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	mod "github.com/jbenet/go-ipfs/unixfs/mod"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

type File interface {
	io.ReadWriteCloser
	io.WriterAt
	Seek(int64, int) (int64, error)
	Size() int64
	Flush() error
	GetNode() (*dag.Node, error)
}

type file struct {
	parent childCloser
	dserv  dag.DAGService
	node   *dag.Node

	name     string
	openMode int

	refLk sync.Mutex
	ref   int
	wref  bool

	mod *mod.DagModifier
}

func NewFile(name string, node *dag.Node, parent childCloser, dserv dag.DAGService) (*file, error) {
	dmod, err := mod.NewDagModifier(context.TODO(), node, dserv, nil, chunk.DefaultSplitter)
	if err != nil {
		return nil, err
	}

	return &file{
		parent: parent,
		dserv:  dserv,
		node:   node,
		name:   name,
		mod:    dmod,
	}, nil
}

func (fi *file) Write(b []byte) (int, error) {
	return fi.mod.Write(b)
}

func (fi *file) Read(b []byte) (int, error) {
	return fi.mod.Read(b)
}

func (fi *file) Close() error {
	err := fi.mod.Flush()
	if err != nil {
		return err
	}

	err = fi.parent.closeChildFile(fi.name)
	if err != nil {
		return err
	}

	// Release potentially held resources
	fi.mod = nil
	fi.dserv = nil
	fi.node = nil
	fi.parent = nil
	return nil
}

func (fi *file) Flush() error {
	return fi.mod.Flush()
}

func (fi *file) withMode(mode int) File {
	if mode == os.O_RDONLY {
		return &readOnlyFile{fi}
	}
	return fi
}

func (fi *file) Seek(offset int64, whence int) (int64, error) {
	return fi.mod.Seek(offset, whence)
}

func (fi *file) WriteAt(b []byte, at int64) (int, error) {
	return fi.mod.WriteAt(b, at)
}

func (fi *file) Size() int64 {
	return int64(fi.mod.Size())
}

func (fi *file) GetNode() (*dag.Node, error) {
	return fi.mod.GetNode()
}

type readOnlyFile struct {
	*file
}

func (ro *readOnlyFile) Write([]byte) (int, error) {
	return 0, errors.New("permission denied: file readonly")
}
