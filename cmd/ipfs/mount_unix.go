// +build linux darwin freebsd

package main

import (
	"errors"
	"fmt"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	"github.com/jbenet/go-ipfs/daemon"
	ipns "github.com/jbenet/go-ipfs/fuse/ipns"
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

func init() {
	cmdIpfsMount.Flag.String("n", "", "specify a mountpoint for ipns")
}

func mountCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 || len(inp[0]) == 0 {
		u.POut(c.Long)
		return nil
	}

	conf, err := getConfigDir(c.Parent)
	if err != nil {
		fmt.Println("Couldnt get config dir")
		return err
	}
	n, err := localNode(conf, true)
	if err != nil {
		fmt.Println("Local node creation failed.")
		return err
	}

	// launch the API RPC endpoint.
	if n.Config.Addresses.API == "" {
		return errors.New("no config.RPCAddress endpoint supplied")
	}

	maddr, err := ma.NewMultiaddr(n.Config.Addresses.API)
	if err != nil {
		return err
	}

	dl, err := daemon.NewDaemonListener(n, maddr, conf)
	if err != nil {
		fmt.Println("Failed to create daemon listener.")
		return err
	}
	go dl.Listen()
	defer dl.Close()

	mp := inp[0]
	fmt.Printf("Mounting at %s\n", mp)

	var ipnsDone chan struct{}
	ns, ok := c.Flag.Lookup("n").Value.Get().(string)
	if ok {
		ipnsDone = make(chan struct{})
		go func() {
			err = ipns.Mount(n, ns, mp)
			if err != nil {
				fmt.Printf("Error mounting ipns: %s\n", err)
			}
			ipnsDone <- struct{}{}
		}()
	}

	err = rofs.Mount(n, mp)
	if ipnsDone != nil {
		<-ipnsDone
	}
	return err
}
