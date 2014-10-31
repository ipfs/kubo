// +build linux darwin freebsd

package main

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"

	core "github.com/jbenet/go-ipfs/core"
	ipns "github.com/jbenet/go-ipfs/fuse/ipns"
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
	if err := osxFuseCheck(); err != nil {
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
	fsdone := mountIpfs(cc.node, fsdir)

	// get default mount points
	nsdir := cc.node.Config.Mounts.IPNS
	if val, ok := c.Flag.Lookup("n").Value.Get().(string); ok && val != "" {
		nsdir = val
	}
	nsdone := mountIpns(cc.node, nsdir, fsdir)

	// wait till mounts are done.
	err1 := <-fsdone
	err2 := <-nsdone

	if err1 != nil {
		return err1
	}
	return err2
}

func mountIpfs(node *core.IpfsNode, fsdir string) <-chan error {
	done := make(chan error)
	fmt.Printf("mounting ipfs at %s\n", fsdir)

	go func() {
		err := rofs.Mount(node, fsdir)
		done <- err
		close(done)
	}()

	return done
}

func mountIpns(node *core.IpfsNode, nsdir, fsdir string) <-chan error {
	if nsdir == "" {
		return nil
	}
	done := make(chan error)
	fmt.Printf("mounting ipns at %s\n", nsdir)

	go func() {
		err := ipns.Mount(node, nsdir, fsdir)
		done <- err
		close(done)
	}()

	return done
}

func osxFuseCheck() error {
	// on OSX, check FUSE version.
	if runtime.GOOS != "darwin" {
		return nil
	}

	ov, err := syscall.Sysctl("osxfuse.version.number")
	if err != nil {
		return err
	}

	if strings.HasPrefix(ov, "2.7.") || strings.HasPrefix(ov, "2.8.") {
		return nil
	}

	return fmt.Errorf("osxfuse version %s not supported.\n%s\n%s", ov,
		"Older versions of osxfuse have kernel panic bugs; please upgrade!",
		"https://github.com/jbenet/go-ipfs/issues/177")
}
