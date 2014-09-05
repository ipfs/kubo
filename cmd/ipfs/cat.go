package main

import (
	"fmt"
	"io"
	"os"

	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	dag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
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

	n, err := localNode(false)
	if err != nil {
		return err
	}

	for _, fn := range inp {
		nd, err := n.Resolver.ResolvePath(fn)
		if err != nil {
			return err
		}

		err = ExpandDag(nd, n.Blocks)
		if err != nil {
			return err
		}

		read, err := dag.NewDagReader(nd)
		if err != nil {
			fmt.Println(err)
			continue
		}

		_, err = io.Copy(os.Stdout, read)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}
	return nil
}

// Expand all subnodes in this dag so printing can occur without error
//TODO: this needs to be done MUCH better in a somewhat asynchronous way.
//also should be moved elsewhere.
func ExpandDag(nd *dag.Node, bs *bserv.BlockService) error {
	for _, lnk := range nd.Links {
		if lnk.Node == nil {
			blk, err := bs.GetBlock(u.Key(lnk.Hash))
			if err != nil {
				return err
			}
			lnk.Node = &dag.Node{Data: dag.WrapData(blk.Data)}
		}
	}
	return nil
}
