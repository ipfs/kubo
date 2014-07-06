package main

import (
	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
)

var cmdIpfsRefs = &commander.Command{
	UsageLine: "refs",
	Short:     "List link hashes from an object.",
	Long: `ipfs refs <ipfs-path> - List link hashes from an object..

    Retrieves the object named by <ipfs-path> and displays the link
    hashes it contains, with the following format:

    <link base58 hash>

    Note: list all refs recursively with -r.

`,
	Run:  refCmd,
	Flag: *flag.NewFlagSet("ipfs-refs", flag.ExitOnError),
}

func init() {
	cmdIpfsRefs.Flag.Bool("r", false, "recursive: list refs recursively")
	cmdIpfsRefs.Flag.Bool("u", false, "unique: list each ref only once")
}

func refCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}

	n, err := localNode()
	if err != nil {
		return err
	}

	recursive := c.Flag.Lookup("r").Value.Get().(bool)
	unique := c.Flag.Lookup("u").Value.Get().(bool)
	refsSeen := map[u.Key]bool{}

	printRef := func(h mh.Multihash) {
		if unique {
			_, found := refsSeen[u.Key(h)]
			if found {
				return
			}
			refsSeen[u.Key(h)] = true
		}

		u.POut("%s\n", h.B58String())
	}

	var printRefs func(h mh.Multihash, recursive bool)
	printRefs = func(h mh.Multihash, recursive bool) {
		nd, err := n.DAG.Get(u.Key(h))
		if err != nil {
			u.PErr("error: cannot retrieve %s (%s)\n", h.B58String(), err)
			return
		}

		for _, link := range nd.Links {
			printRef(link.Hash)
			if recursive {
				printRefs(link.Hash, recursive)
			}
		}
	}

	for _, fn := range inp {
		// for now only hashes, no path resolution
		h, err := mh.FromB58String(fn)
		if err != nil {
			return err
		}

		printRefs(h, recursive)
	}
	return nil
}
