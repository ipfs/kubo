// +build linux darwin freebsd

package main

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"

	core "github.com/jbenet/go-ipfs/core"
	ipns "github.com/jbenet/go-ipfs/fuse/ipns"
	mount "github.com/jbenet/go-ipfs/fuse/mount"
	rofs "github.com/jbenet/go-ipfs/fuse/readonly"
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
	cmdIpfsMount.Flag.String("f", "", "specify a mountpoint for ipfs")
	cmdIpfsMount.Flag.String("n", "", "specify a mountpoint for ipns")
}

func mountCmd(c *commander.Command, inp []string) error {
	if err := platformFuseChecks(); err != nil {
		return err
	}

	cc, err := setupCmdContext(c, true)
	if err != nil {
		return err
	}
	defer cc.daemon.Close()

	// update fsdir with flag.
	fsdir := cc.node.Config.Mounts.IPFS
	if val, ok := c.Flag.Lookup("f").Value.Get().(string); ok && val != "" {
		fsdir = val
	}

	// get default mount points
	nsdir := cc.node.Config.Mounts.IPNS
	if val, ok := c.Flag.Lookup("n").Value.Get().(string); ok && val != "" {
		nsdir = val
	}

	return doMount(cc.node, fsdir, nsdir)
}

func doMount(node *core.IpfsNode, fsdir, nsdir string) error {

	// this sync stuff is so that both can be mounted simultaneously.
	var fsmount mount.Mount
	var nsmount mount.Mount
	var err1 error
	var err2 error

	done := make(chan struct{})

	go func() {
		fsmount, err1 = rofs.Mount(node, fsdir)
		done <- struct{}{}
	}()

	go func() {
		nsmount, err2 = ipns.Mount(node, nsdir, fsdir)
		done <- struct{}{}
	}()

	<-done
	<-done

	if err1 != nil || err2 != nil {
		fsmount.Close()
		nsmount.Close()
		if err1 != nil {
			return err1
		} else {
			return err2
		}
	}

	// setup node state, so that it can be cancelled
	node.Mounts.Ipfs = fsmount
	node.Mounts.Ipns = nsmount
	return nil
}

var platformFuseChecks = func() error {
	return nil
}
