package main

import (
	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
)

var cmdIpfsCat = &commander.Command{
	UsageLine: "cat",
	Short:     "Show ipfs object data.",
	Long: `ipfs cat <ipfs-path> - Show ipfs object data.

    Retrieves the object named by <ipfs-path> and displays the Data
    it contains.
`,
	Run:  catCmd,
	Flag: *flag.NewFlagSet("ipfs-cat", flag.ExitOnError),
}

func catCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}

	// for now only hashes, no path resolution
	h, err := mh.FromB58String(inp[0])
	if err != nil {
		return err
	}

	n, err := localNode()
	if err != nil {
		return err
	}

	nd, err := n.GetDagNode(u.Key(h))
	if err != nil {
		return err
	}

	u.POut("%s", nd.Data)
	return nil
}
