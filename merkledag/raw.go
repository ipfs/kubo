package merkledag

import (
	"github.com/ipfs/go-ipfs/blocks"

	cid "gx/ipfs/QmXUuRadqDq5BuFWzVU6VuKaSjTcNm1gNCtLvvP1TJCW4z/go-cid"
	node "gx/ipfs/QmZx42H5khbVQhV5odp66TApShV4XCujYazcvYduZ4TroB/go-ipld-node"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
)

type RawNode struct {
	blocks.Block
}

func NewRawNode(data []byte) *RawNode {
	h := u.Hash(data)
	c := cid.NewCidV1(cid.Raw, h)
	blk, _ := blocks.NewBlockWithCid(data, c)

	return &RawNode{blk}
}

func (rn *RawNode) Links() []*node.Link {
	return nil
}

func (rn *RawNode) Resolve(path []string) (*node.Link, []string, error) {
	return nil, nil, ErrLinkNotFound
}

func (rn *RawNode) Tree() []string {
	return nil
}

func (rn *RawNode) Size() (uint64, error) {
	return uint64(len(rn.RawData())), nil
}

func (rn *RawNode) Stat() (*node.NodeStat, error) {
	return &node.NodeStat{
		CumulativeSize: len(rn.RawData()),
		DataSize:       len(rn.RawData()),
	}, nil
}

var _ node.Node = (*RawNode)(nil)
