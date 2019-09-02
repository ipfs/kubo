package fsnodes

import (
	"context"

	"github.com/djdv/p9/p9"
	"github.com/djdv/p9/unimplfs"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

type FSNode interface {
	corepath.Path
	//RWLocker
	Stat() (p9.QID, error)
}

const ( //device
	dMemory = iota
	dIPFS
)

const ( //FS namespaces
	nRoot = "root"
)

var _ p9.File = (*Base)(nil)

//var _ FSNode = (*Base)(nil)

//TODO: docs
// Base is a foundational node, intended to be embedded/extended
type Base struct {
	unimplfs.NoopFile
	p9.DefaultWalkGetAttr
	Qid      p9.QID
	meta     p9.Attr
	metaMask p9.AttrMask

	Ctx    context.Context
	Logger logging.EventLogger
}

type IPFSBase struct {
	Base

	//Path corepath.Path
	Path corepath.Resolved
	core coreiface.CoreAPI
}
