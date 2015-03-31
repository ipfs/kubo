package ipnsfs

import (
	"sync"

	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mod "github.com/ipfs/go-ipfs/unixfs/mod"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

type File struct {
	parent childCloser
	fs     *Filesystem

	name       string
	hasChanges bool

	mod  *mod.DagModifier
	lock sync.Mutex
}

// NewFile returns a NewFile object with the given parameters
func NewFile(name string, node *dag.Node, parent childCloser, fs *Filesystem) (*File, error) {
	dmod, err := mod.NewDagModifier(context.Background(), node, fs.dserv, fs.pins.GetManual(), chunk.DefaultSplitter)
	if err != nil {
		return nil, err
	}

	return &File{
		fs:     fs,
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
	defer fi.Unlock()
	if fi.hasChanges {
		err := fi.mod.Sync()
		if err != nil {
			return err
		}

		nd, err := fi.mod.GetNode()
		if err != nil {
			return err
		}

		fi.Unlock()
		err = fi.parent.closeChild(fi.name, nd)
		fi.Lock()
		if err != nil {
			return err
		}

		fi.hasChanges = false
	}

	return nil
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
