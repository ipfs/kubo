package main

import (
	"fmt"
	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	rofs "github.com/jbenet/go-ipfs/fuse/readonly"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsMount = &commander.Command{
	UsageLine: "mount",
	Short:     "Mount an ipfs read-only mountpoint.",
	Long: `ipfs mount <os-path> - Mount an ipfs read-only mountpoint.

    Mount ipfs at a read-only mountpoint on the OS. All ipfs objects
    will be accessible under that directory. Note that the root will
    not be listable, as it is virtual. Accessing known paths directly.

`,
	Run:  mountCmd,
	Flag: *flag.NewFlagSet("ipfs-mount", flag.ExitOnError),
}

func mountCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 || len(inp[0]) == 0 {
		u.POut(c.Long)
		return nil
	}

	n, err := localNode()
	if err != nil {
		return err
	}

	mp := inp[0]
	fmt.Printf("Mounting at %s\n", mp)

	return rofs.Mount(n, mp)
}
