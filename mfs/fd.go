package mfs

import (
	"context"
	"fmt"
	"io"

	mod "github.com/ipfs/go-ipfs/unixfs/mod"

	node "gx/ipfs/QmNwUEK7QbwSqyKBu3mMtToo8SUc6wQJ7gdZq4gGGJqfnf/go-ipld-format"
)

type state uint8

const (
	stateFlushed state = iota
	stateSynced
	stateDirty
)

type FileDescriptor interface {
	io.Reader
	CtxReadFull(context.Context, []byte) (int, error)

	io.Writer
	io.WriterAt

	io.Closer
	io.Seeker

	Truncate(int64) error
	Size() (int64, error)
	Sync() error
	Flush() error
}

type fileDescriptor struct {
	inode *File
	mod   *mod.DagModifier
	mode  mode
	sync  bool

	state state
}

// Size returns the size of the file referred to by this descriptor
func (fi *fileDescriptor) Size() (int64, error) {
	return fi.mod.Size()
}

// Truncate truncates the file to size
func (fi *fileDescriptor) Truncate(size int64) error {
	if !fi.mode.CanWrite() {
		return fmt.Errorf("truncate failed: file is %s", fi.mode)
	}
	fi.state = stateDirty
	return fi.mod.Truncate(size)
}

// Write writes the given data to the file at its current offset
func (fi *fileDescriptor) Write(b []byte) (int, error) {
	if !fi.mode.CanWrite() {
		return 0, fmt.Errorf("write failed: file is %s", fi.mode)
	}
	fi.state = stateDirty
	return fi.mod.Write(b)
}

// Read reads into the given buffer from the current offset
func (fi *fileDescriptor) Read(b []byte) (int, error) {
	if !fi.mode.CanRead() {
		return 0, fmt.Errorf("read failed: file is %s", fi.mode)
	}
	return fi.mod.Read(b)
}

// Read reads into the given buffer from the current offset
func (fi *fileDescriptor) CtxReadFull(ctx context.Context, b []byte) (int, error) {
	if !fi.mode.CanRead() {
		return 0, fmt.Errorf("read failed: file is %s", fi.mode)
	}
	return fi.mod.CtxReadFull(ctx, b)
}

// Close flushes, then propogates the modified dag node up the directory structure
// and signals a republish to occur
func (fi *fileDescriptor) Close() error {
	if fi.mode == Closed {
		return ErrClosed
	}
	if fi.mode.CanWrite() {
		defer fi.inode.desclock.Unlock()
	} else if fi.mode.CanRead() {
		defer fi.inode.desclock.RUnlock()
	}
	fi.mode = Closed

	return fi.flushUp(fi.sync)
}

func (fi *fileDescriptor) Sync() error {
	if fi.mode == Closed {
		return ErrClosed
	}
	return fi.flushUp(false)
}

func (fi *fileDescriptor) Flush() error {
	if fi.mode == Closed {
		return ErrClosed
	}
	return fi.flushUp(true)
}

// flushUp syncs the file and adds it to the dagservice
// it *must* be called with the File's lock taken
func (fi *fileDescriptor) flushUp(fullsync bool) error {
	var nd node.Node
	switch fi.state {
	case stateDirty:
		// calls mod.Sync internally.
		var err error
		nd, err = fi.mod.GetNode()
		if err != nil {
			return err
		}

		_, err = fi.inode.dserv.Add(nd)
		if err != nil {
			return err
		}

		fi.inode.nodelk.Lock()
		fi.inode.node = nd
		fi.inode.nodelk.Unlock()
		fi.state = stateSynced
		fallthrough
	case stateSynced:
		if !fullsync {
			return nil
		}
		if nd == nil {
			fi.inode.nodelk.RLock()
			nd = fi.inode.node
			fi.inode.nodelk.RUnlock()
		}

		if err := fi.inode.parent.closeChild(fi.inode.name, nd, fullsync); err != nil {
			return err
		}
		fi.state = stateFlushed
		return nil
	case stateFlushed:
		return nil
	default:
		panic("invalid state")
	}
}

// Seek implements io.Seeker
func (fi *fileDescriptor) Seek(offset int64, whence int) (int64, error) {
	if fi.mode == Closed {
		return 0, ErrClosed
	}
	return fi.mod.Seek(offset, whence)
}

// Write At writes the given bytes at the offset 'at'
func (fi *fileDescriptor) WriteAt(b []byte, at int64) (int, error) {
	if !fi.mode.CanWrite() {
		return 0, fmt.Errorf("write-at failed: file is %s", fi.mode)
	}
	fi.state = stateDirty
	return fi.mod.WriteAt(b, at)
}
