package posinfo

import (
	"os"

	node "gx/ipfs/QmVHxZ8ovAuHiHTbJa68budGYAqmMUzb1bqDW1SVb6y5M9/go-ipld-format"
)

type PosInfo struct {
	Offset   uint64
	FullPath string
	Stat     os.FileInfo // can be nil
}

type FilestoreNode struct {
	node.Node
	PosInfo *PosInfo
}
