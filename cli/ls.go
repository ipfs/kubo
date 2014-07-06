package main

import (
	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
)

var cmdIpfsLs = &commander.Command{
	UsageLine: "ls",
	Short:     "List links from an object.",
	Long: `ipfs ls <ipfs-path> - List links from an object.

    Retrieves the object named by <ipfs-path> and displays the links
    it contains, with the following format:

    <link base58 hash> <link size in bytes> <link name>

`,
	Run:  lsCmd,
	Flag: *flag.NewFlagSet("ipfs-ls", flag.ExitOnError),
}

func lsCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}

	n, err := localNode()
	if err != nil {
		return err
	}

	for _, fn := range inp {
		// for now only hashes, no path resolution
		h, err := mh.FromB58String(fn)
		if err != nil {
			return err
		}

		nd, err := n.GetDagNode(u.Key(h))
		if err != nil {
			return err
		}

		for _, link := range nd.Links {
			u.POut("%s %d %s\n", link.Hash.B58String(), link.Size, link.Name)
		}
	}
	return nil
}
