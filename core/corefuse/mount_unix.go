// +build linux darwin freebsd

package corefuse

import (
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	ipns "github.com/jbenet/go-ipfs/fuse/ipns"
	mount "github.com/jbenet/go-ipfs/fuse/mount"
	rofs "github.com/jbenet/go-ipfs/fuse/readonly"
)

// fuseNoDirectory used to check the returning fuse error
const fuseNoDirectory = "fusermount: failed to access mountpoint"

func Mount(node *core.IpfsNode, fsdir, nsdir string) error {
	// check if we already have live mounts.
	// if the user said "Mount", then there must be something wrong.
	// so, close them and try again.
	if node.Mounts.Ipfs != nil {
		node.Mounts.Ipfs.Unmount()
	}
	if node.Mounts.Ipns != nil {
		node.Mounts.Ipns.Unmount()
	}

	if err := platformFuseChecks(); err != nil {
		return err
	}

	var err error
	if err = doMount(node, fsdir, nsdir); err != nil {
		return err
	}

	return nil
}

var platformFuseChecks = func() error {
	return nil
}

func doMount(node *core.IpfsNode, fsdir, nsdir string) error {
	fmtFuseErr := func(err error) error {
		s := err.Error()
		if strings.Contains(s, fuseNoDirectory) {
			s = strings.Replace(s, `fusermount: "fusermount:`, "", -1)
			s = strings.Replace(s, `\n", exit status 1`, "", -1)
			return cmds.ClientError(s)
		}
		return err
	}

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
		log.Infof("error mounting: %s %s", err1, err2)
		if fsmount != nil {
			fsmount.Unmount()
		}
		if nsmount != nil {
			nsmount.Unmount()
		}

		if err1 != nil {
			return fmtFuseErr(err1)
		} else {
			return fmtFuseErr(err2)
		}
	}

	// setup node state, so that it can be cancelled
	node.Mounts.Ipfs = fsmount
	node.Mounts.Ipns = nsmount
	return nil
}
