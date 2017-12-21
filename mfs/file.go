package mfs

import (
	"context"
	"fmt"
	"sync"

	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	mod "github.com/ipfs/go-ipfs/unixfs/mod"

	node "gx/ipfs/QmNwUEK7QbwSqyKBu3mMtToo8SUc6wQJ7gdZq4gGGJqfnf/go-ipld-format"
)

type File struct {
	parent childCloser

	name string

	desclock sync.RWMutex

	dserv  dag.DAGService
	node   node.Node
	nodelk sync.Mutex

	RawLeaves bool
}

// NewFile returns a NewFile object with the given parameters.  If the
// Cid version is non-zero RawLeaves will be enabled.
func NewFile(name string, node node.Node, parent childCloser, dserv dag.DAGService) (*File, error) {
	fi := &File{
		dserv:  dserv,
		parent: parent,
		name:   name,
		node:   node,
	}
	if node.Cid().Prefix().Version > 0 {
		fi.RawLeaves = true
	}
	return fi, nil
}

type mode uint8

const (
	Closed        mode = 0x0 // No access. Needs to be 0.
	ModeRead      mode = 1 << 0
	ModeWrite     mode = 1 << 1
	ModeReadWrite mode = ModeWrite | ModeRead
)

func (m mode) CanRead() bool {
	return m&ModeRead != 0
}
func (m mode) CanWrite() bool {
	return m&ModeWrite != 0
}

func (m mode) String() string {
	switch m {
	case ModeWrite:
		return "write-only"
	case ModeRead:
		return "read-only"
	case ModeReadWrite:
		return "read-write"
	case Closed:
		return "closed"
	default:
		return "invalid"
	}
}

func (fi *File) Open(mode mode, sync bool) (FileDescriptor, error) {
	fi.nodelk.Lock()
	node := fi.node
	fi.nodelk.Unlock()

	switch node := node.(type) {
	case *dag.ProtoNode:
		fsn, err := ft.FSNodeFromBytes(node.Data())
		if err != nil {
			return nil, err
		}

		switch fsn.Type {
		default:
			return nil, fmt.Errorf("unsupported fsnode type for 'file'")
		case ft.TSymlink:
			return nil, fmt.Errorf("symlinks not yet supported")
		case ft.TFile, ft.TRaw:
			// OK case
		}
	case *dag.RawNode:
		// Ok as well.
	}

	if mode > 0x3 {
		// TODO: support other modes
		return nil, fmt.Errorf("mode not supported")
	}

	if mode.CanWrite() {
		fi.desclock.Lock()
	} else if mode.CanRead() {
		fi.desclock.RLock()
	} else {
		// For now, need to open with either read or write perm.
		return nil, fmt.Errorf("mode not supported")
	}

	dmod, err := mod.NewDagModifier(context.TODO(), node, fi.dserv, chunk.DefaultSplitter)
	if err != nil {
		return nil, err
	}
	dmod.RawLeaves = fi.RawLeaves

	return &fileDescriptor{
		inode: fi,
		mode:  mode,
		sync:  sync,
		mod:   dmod,
	}, nil
}

// Size returns the size of this file
func (fi *File) Size() (int64, error) {
	fi.nodelk.Lock()
	defer fi.nodelk.Unlock()
	switch nd := fi.node.(type) {
	case *dag.ProtoNode:
		pbd, err := ft.FromBytes(nd.Data())
		if err != nil {
			return 0, err
		}
		return int64(pbd.GetFilesize()), nil
	case *dag.RawNode:
		return int64(len(nd.RawData())), nil
	default:
		return 0, fmt.Errorf("unrecognized node type in mfs/file.Size()")
	}
}

// GetNode returns the dag node associated with this file
func (fi *File) GetNode() (node.Node, error) {
	fi.nodelk.Lock()
	defer fi.nodelk.Unlock()
	return fi.node, nil
}

func (fi *File) Flush() error {
	// open the file in fullsync mode
	fd, err := fi.Open(ModeWrite, true)
	if err != nil {
		return err
	}

	defer fd.Close()

	return fd.Flush()
}

func (fi *File) Sync() error {
	// just being able to take the writelock means the descriptor is synced
	fi.desclock.Lock()
	fi.desclock.Unlock()
	return nil
}

// Type returns the type FSNode this is
func (fi *File) Type() NodeType {
	return TFile
}
