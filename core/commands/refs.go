package commands

import (
	"io"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	"github.com/jbenet/go-ipfs/core"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

func Refs(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	unique, ok := opts["u"].(bool)
	if !ok {
		unique = false
	}

	recursive, ok := opts["r"].(bool)
	if !ok {
		recursive = false
	}

	var refsSeen map[u.Key]bool
	if unique {
		refsSeen = make(map[u.Key]bool)
	}

	for _, fn := range args {
		nd, err := n.Resolver.ResolvePath(fn)
		if err != nil {
			return err
		}

		printRefs(n, nd, refsSeen, recursive)
	}
	return nil
}

func printRefs(n *core.IpfsNode, nd *mdag.Node, refSeen map[u.Key]bool, recursive bool) {
	for _, link := range nd.Links {
		printRef(link.Hash, refSeen)

		if recursive {
			nd, err := n.DAG.Get(u.Key(link.Hash))
			if err != nil {
				u.PErr("error: cannot retrieve %s (%s)\n", link.Hash.B58String(), err)
				return
			}

			printRefs(n, nd, refSeen, recursive)
		}
	}
}

func printRef(h mh.Multihash, refsSeen map[u.Key]bool) {
	if refsSeen != nil {
		_, found := refsSeen[u.Key(h)]
		if found {
			return
		}
		refsSeen[u.Key(h)] = true
	}

	u.POut("%s\n", h.B58String())
}
