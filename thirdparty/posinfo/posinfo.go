package posinfo

import (
	"os"

	node "gx/ipfs/QmUsVJ7AEnGyjX8YWnrwq9vmECVGwBQNAKPpgz5KSg8dcq/go-ipld-node"
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
