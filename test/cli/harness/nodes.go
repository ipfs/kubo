package harness

import (
	. "github.com/ipfs/kubo/test/cli/testutils"
)

// Nodes is a collection of Kubo nodes along with operations on groups of nodes.
type Nodes []*Node

func (n Nodes) Init(args ...string) Nodes {
	ForEachPar(n, func(node *Node) {
		node.Init(args...)
	})
	return n
}

func (n Nodes) Connect() Nodes {
	for _, node := range n {
		node := node
		ForEachPar(n, func(otherNode *Node) {
			if node.ID == otherNode.ID {
				return
			}
			node.Connect(otherNode)
		})
	}
	return n
}

func (n Nodes) StartDaemons() Nodes {
	ForEachPar(n, func(node *Node) {
		node.StartDaemon()
	})
	return n
}

func (n Nodes) StopDaemons() Nodes {
	ForEachPar(n, func(node *Node) {
		node.StopDaemon()
	})
	return n
}
