package mfs

import (
	ipld "gx/ipfs/QmWi2BYBL5gJ3CiAiQchg6rn1A8iBsrWy51EYxvHVjFvLb/go-ipld-format"
)

// Inode abstracts the common characteristics of the MFS `File`
// and `Directory`. All of its attributes are initialized at
// creation and can't be modified.
type Inode interface {

	// Name of this `Inode` in the MFS path (the same value
	// is also stored as the name of the DAG link).
	Name() string

	// Parent directory of this `Inode` (which may be the `Root`).
	Parent() childCloser

	// DagService used to store modifications made to the contents
	// of the file or directory the `Inode` belongs to.
	DagService() ipld.DAGService
}

// inode implements the `Inode` interface.
type inode struct {
	name       string
	parent     childCloser
	dagService ipld.DAGService
}

// NewInode creates a new `inode` structure and returns its pointer
// as the `Inode` interface.
func NewInode(name string, parent childCloser, dagService ipld.DAGService) Inode {
	return &inode{
		name:       name,
		parent:     parent,
		dagService: dagService,
	}
}

// Name implements the `Inode` interface.
func (i *inode) Name() string {
	return i.name
}

// Parent implements the `Inode` interface.
func (i *inode) Parent() childCloser {
	return i.parent
}

// DagService implements the `Inode` interface.
func (i *inode) DagService() ipld.DAGService {
	return i.dagService
}
