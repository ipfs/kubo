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
	Size() (int64, error)
	Flush() error
	Truncate(int64) error
	FSNode
}

type file struct {
	parent childCloser
	fs     *Filesystem

	name       string
	hasChanges bool

	// TODO: determine whether or not locking here is actually required...
	lk  sync.Mutex
	mod *mod.DagModifier
}

func NewFile(name string, node *dag.Node, parent childCloser, fs *Filesystem) (*file, error) {
	dmod, err := mod.NewDagModifier(context.TODO(), node, fs.dserv, fs.pins.GetManual(), chunk.DefaultSplitter)
	if err != nil {
		return nil, err
	}

	return &file{
		fs:     fs,
		parent: parent,
		name:   name,
		mod:    dmod,
	}, nil
}

func (fi *file) Write(b []byte) (int, error) {
	fi.lk.Lock()
	defer fi.lk.Unlock()
	fi.hasChanges = true
	return fi.mod.Write(b)
}

func (fi *file) Read(b []byte) (int, error) {
	fi.lk.Lock()
	defer fi.lk.Unlock()
	return fi.mod.Read(b)
}

func (fi *file) Close() error {
	fi.lk.Lock()
	defer fi.lk.Unlock()
	if fi.hasChanges {
		err := fi.mod.Flush()
		if err != nil {
			return err
		}

		nd, err := fi.mod.GetNode()
		if err != nil {
			return err
		}

		err = fi.parent.closeChild(fi.name, nd)
		if err != nil {
			return err
		}

		fi.hasChanges = false
	}

	return nil
}

func (fi *file) Flush() error {
	fi.lk.Lock()
	defer fi.lk.Unlock()
	return fi.mod.Flush()
}

func (fi *file) withMode(mode int) File {
	if mode == os.O_RDONLY {
		return &readOnlyFile{fi}
	}
	return fi
}

func (fi *file) Seek(offset int64, whence int) (int64, error) {
	fi.lk.Lock()
	defer fi.lk.Unlock()
	return fi.mod.Seek(offset, whence)
}

func (fi *file) WriteAt(b []byte, at int64) (int, error) {
	fi.lk.Lock()
	defer fi.lk.Unlock()
	fi.hasChanges = true
	return fi.mod.WriteAt(b, at)
}

func (fi *file) Size() (int64, error) {
	fi.lk.Lock()
	defer fi.lk.Unlock()
	return fi.mod.Size()
}

func (fi *file) GetNode() (*dag.Node, error) {
	fi.lk.Lock()
	defer fi.lk.Unlock()
	return fi.mod.GetNode()
}

func (fi *file) Truncate(size int64) error {
	fi.lk.Lock()
	defer fi.lk.Unlock()
	fi.hasChanges = true
	return fi.mod.Truncate(size)
}

func (fi *file) Type() NodeType {
	return TFile
}

type readOnlyFile struct {
	*file
}

func (ro *readOnlyFile) Write([]byte) (int, error) {
	return 0, errors.New("permission denied: file readonly")
}
