package mfs

import (
	"sync"

	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mod "github.com/ipfs/go-ipfs/unixfs/mod"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

type File struct {
	parent childCloser

	name       string
	hasChanges bool

	dserv dag.DAGService
	mod   *mod.DagModifier
	lock  sync.Mutex
}

// NewFile returns a NewFile object with the given parameters
func NewFile(name string, node *dag.Node, parent childCloser, dserv dag.DAGService) (*File, error) {
	dmod, err := mod.NewDagModifier(context.Background(), node, dserv, chunk.DefaultSplitter)
	if err != nil {
		return nil, err
	}

	return &File{
		dserv:  dserv,
		parent: parent,
		name:   name,
		mod:    dmod,
	}, nil
}

// Write writes the given data to the file at its current offset
func (fi *File) Write(b []byte) (int, error) {
	fi.Lock()
	defer fi.Unlock()
	fi.hasChanges = true
	return fi.mod.Write(b)
}

// Read reads into the given buffer from the current offset
func (fi *File) Read(b []byte) (int, error) {
	fi.Lock()
	defer fi.Unlock()
	return fi.mod.Read(b)
}

// Read reads into the given buffer from the current offset
func (fi *File) CtxReadFull(ctx context.Context, b []byte) (int, error) {
	fi.Lock()
	defer fi.Unlock()
	return fi.mod.CtxReadFull(ctx, b)
}

// Close flushes, then propogates the modified dag node up the directory structure
// and signals a republish to occur
func (fi *File) Close() error {
	fi.Lock()
	if fi.hasChanges {
		err := fi.mod.Sync()
		if err != nil {
			return err
		}

		fi.hasChanges = false

		// explicitly stay locked for flushUp call,
		// it will manage the lock for us
		return fi.flushUp()
	}

	return nil
}

// flushUp syncs the file and adds it to the dagservice
// it *must* be called with the File's lock taken
func (fi *File) flushUp() error {
	nd, err := fi.mod.GetNode()
	if err != nil {
		fi.Unlock()
		return err
	}

	_, err = fi.dserv.Add(nd)
	if err != nil {
		fi.Unlock()
		return err
	}

	name := fi.name
	parent := fi.parent

	// explicit unlock *only* before closeChild call
	fi.Unlock()
	return parent.closeChild(name, nd)
}

// Sync flushes the changes in the file to disk
func (fi *File) Sync() error {
	fi.Lock()
	defer fi.Unlock()
	return fi.mod.Sync()
}

// Seek implements io.Seeker
func (fi *File) Seek(offset int64, whence int) (int64, error) {
	fi.Lock()
	defer fi.Unlock()
	return fi.mod.Seek(offset, whence)
}

// Write At writes the given bytes at the offset 'at'
func (fi *File) WriteAt(b []byte, at int64) (int, error) {
	fi.Lock()
	defer fi.Unlock()
	fi.hasChanges = true
	return fi.mod.WriteAt(b, at)
}

// Size returns the size of this file
func (fi *File) Size() (int64, error) {
	fi.Lock()
	defer fi.Unlock()
	return fi.mod.Size()
}

// GetNode returns the dag node associated with this file
func (fi *File) GetNode() (*dag.Node, error) {
	fi.Lock()
	defer fi.Unlock()
	return fi.mod.GetNode()
}

// Truncate truncates the file to size
func (fi *File) Truncate(size int64) error {
	fi.Lock()
	defer fi.Unlock()
	fi.hasChanges = true
	return fi.mod.Truncate(size)
}

// Type returns the type FSNode this is
func (fi *File) Type() NodeType {
	return TFile
}

// Lock the file
func (fi *File) Lock() {
	fi.lock.Lock()
}

// Unlock the file
func (fi *File) Unlock() {
	fi.lock.Unlock()
}
