package main

import (
	"fmt"

	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
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

		fmt.Println("Printing Data!")
		_, err = fmt.Printf("%s", nd.Data)
		if err != nil {
			return err
		}

		fmt.Println("Printing child nodes:")
		for _, subn := range nd.Links {
			k := u.Key(subn.Hash)
			blk, err := n.Blocks.GetBlock(k)
			fmt.Printf("Getting link: %s\n", k.Pretty())
			if err != nil {
				return err
			}
			fmt.Println(string(blk.Data))
		}
	}
	return nil
}
