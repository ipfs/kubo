package ipnsfs

import (
	"sync"

	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	mod "github.com/jbenet/go-ipfs/unixfs/mod"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

type File struct {
	parent childCloser
	fs     *Filesystem

	name       string
	hasChanges bool

	mod  *mod.DagModifier
	lock sync.Mutex
}

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

func (fi *File) Write(b []byte) (int, error) {
	fi.hasChanges = true
	return fi.mod.Write(b)
}

func (fi *File) Read(b []byte) (int, error) {
	fi.lock.Lock()
	defer fi.lock.Unlock()
	return fi.mod.Read(b)
}

func (fi *File) Close() error {
	fi.lock.Lock()
	defer fi.lock.Unlock()
	if fi.hasChanges {
		err := fi.mod.Flush()
		if err != nil {
			return err
		}

		nd, err := fi.mod.GetNode()
		if err != nil {
			return err
		}

		fi.lock.Unlock()
		err = fi.parent.closeChild(fi.name, nd)
		fi.lock.Lock()
		if err != nil {
			return err
		}

		fi.hasChanges = false
	}

	return nil
}

func (fi *File) Flush() error {
	fi.lock.Lock()
	defer fi.lock.Unlock()
	return fi.mod.Flush()
}

func (fi *File) Seek(offset int64, whence int) (int64, error) {
	fi.lock.Lock()
	defer fi.lock.Unlock()
	return fi.mod.Seek(offset, whence)
}

func (fi *File) WriteAt(b []byte, at int64) (int, error) {
	fi.lock.Lock()
	defer fi.lock.Unlock()
	fi.hasChanges = true
	return fi.mod.WriteAt(b, at)
}

func (fi *File) Size() (int64, error) {
	fi.lock.Lock()
	defer fi.lock.Unlock()
	return fi.mod.Size()
}

func (fi *File) GetNode() (*dag.Node, error) {
	fi.lock.Lock()
	defer fi.lock.Unlock()
	return fi.mod.GetNode()
}

func (fi *File) Truncate(size int64) error {
	fi.lock.Lock()
	defer fi.lock.Unlock()
	fi.hasChanges = true
	return fi.mod.Truncate(size)
}

func (fi *File) Type() NodeType {
	return TFile
}

func (fi *File) Lock() {
	fi.lock.Lock()
}

func (fi *File) Unlock() {
	fi.lock.Unlock()
}
