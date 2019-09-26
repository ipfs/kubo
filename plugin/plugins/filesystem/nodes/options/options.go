package nodeopts

import (
	"github.com/djdv/p9/p9"
	"github.com/jbenet/goprocess"
)

type Options struct {
	Parent  p9.File
	Process goprocess.Process
}

type Option func(*Options)

// if NOT provided, we assume the file system is to be treated as a root, assigning itself as a parent
func Parent(p p9.File) Option {
	return func(ops *Options) {
		ops.Parent = p
	}
}

//TODO: this isn't true yet
// if provided, file systems implemented here will utilize this to create a cascading Close() tree
func Process(p goprocess.Process) Option {
	return func(ops *Options) {
		ops.Process = p
	}
}

func AttachOps(options ...Option) *Options {
	ops := &Options{} // only nil defaults at this time
	for _, op := range options {
		op(ops)
	}
	return ops
}
