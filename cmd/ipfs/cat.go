package main

import (
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

	n, err := localNode()
	if err != nil {
		return err
	}

	for _, fn := range inp {
		nd, err := n.Resolver.ResolvePath(fn)
		if err != nil {
			return err
		}

		u.POut("%s", nd.Data)
	}
	return nil
}
