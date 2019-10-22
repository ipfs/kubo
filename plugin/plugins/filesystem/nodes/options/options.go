package nodeopts

import (
	"github.com/jbenet/goprocess"

	"github.com/ipfs/go-cid"
	fsutils "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/utils"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/go-mfs"
)

type AttachOptions struct {
	Parent     fsutils.WalkRef     // node directly behind self
	Logger     logging.EventLogger // what subsystem you are
	Process    goprocess.Process   // TODO: I documented this somewhere else
	MFSRoot    cid.Cid             // required when attaching to MFS
	MFSPublish mfs.PubFunc
}

type AttachOption func(*AttachOptions)

func AttachOps(options ...AttachOption) *AttachOptions {
	ops := &AttachOptions{
		Logger: logging.Logger("FS"),
	}
	for _, op := range options {
		op(ops)
	}
	return ops
}

// if NOT provided, we assume the file system is to be treated as a root, assigning itself as a parent
func Parent(p fsutils.WalkRef) AttachOption {
	return func(ops *AttachOptions) {
		ops.Parent = p
	}
}

func Logger(l logging.EventLogger) AttachOption {
	return func(ops *AttachOptions) {
		ops.Logger = l
	}
}

//TODO: this isn't true yet
// if provided, file systems implemented here will utilize this to create a cascading Close() tree
func Process(p goprocess.Process) AttachOption {
	return func(ops *AttachOptions) {
		ops.Process = p
	}
}

func MFSRoot(rcid cid.Cid) AttachOption {
	return func(ops *AttachOptions) {
		ops.MFSRoot = rcid
	}
}

func MFSPublish(p mfs.PubFunc) AttachOption {
	return func(ops *AttachOptions) {
		ops.MFSPublish = p
	}
}
