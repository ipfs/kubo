package posinfo

import (
	"os"

	node "gx/ipfs/QmRL2JDEtNzSkEjMgsUBXgmHKeJ7a4V6QoirXHrc93igo2/go-ipld-format"
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
