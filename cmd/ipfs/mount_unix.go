// +build linux darwin freebsd

package main

import (
	"fmt"

	"github.com/spf13/cobra"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	"github.com/jbenet/go-ipfs/daemon"
	rofs "github.com/jbenet/go-ipfs/fuse/readonly"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsMount = &cobra.Command{
	Use: "mount",
	Short:     "Mount an ipfs read-only mountpoint.",
	Long: `ipfs mount <os-path> - Mount an ipfs read-only mountpoint.

    Mount ipfs at a read-only mountpoint on the OS. All ipfs objects
    will be accessible under that directory. Note that the root will
    not be listable, as it is virtual. Accessing known paths directly.

`,
	Run:  mountCmd,
}

func init() {
	CmdIpfs.AddCommand(cmdIpfsMount)
}

func mountCmd(c *cobra.Command, inp []string) {
	if len(inp) < 1 || len(inp[0]) == 0 {
		u.POut(c.Long)
		return
	}

	conf, err := getConfigDir(c)
	if err != nil {
		u.PErr(err.Error())
		return
	}
	n, err := localNode(conf, true)
	if err != nil {
		u.PErr(err.Error())
		return
	}

	// launch the RPC endpoint.
	if n.Config.RPCAddress == "" {
		u.PErr("no config.RPCAddress endpoint supplied")
		return
	}

	maddr, err := ma.NewMultiaddr(n.Config.RPCAddress)
	if err != nil {
		u.PErr(err.Error())
		return
	}

	dl, err := daemon.NewDaemonListener(n, maddr)
	if err != nil {
		u.PErr(err.Error())
		return
	}
	go dl.Listen()
	defer dl.Close()

	mp := inp[0]
	fmt.Printf("Mounting at %s\n", mp)

	err = rofs.Mount(n, mp)
	if err != nil {
		u.PErr(err.Error())
		return
	}
}
