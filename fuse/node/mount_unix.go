//go:build !windows && !openbsd && !netbsd && !plan9 && !nofuse

package node

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	core "github.com/ipfs/kubo/core"
	ipns "github.com/ipfs/kubo/fuse/ipns"
	mfs "github.com/ipfs/kubo/fuse/mfs"
	mount "github.com/ipfs/kubo/fuse/mount"
	rofs "github.com/ipfs/kubo/fuse/readonly"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("node")

// fuseNoDirectory used to check the returning fuse error.
const fuseNoDirectory = "fusermount: failed to access mountpoint"

// fuseExitStatus1 used to check the returning fuse error.
const fuseExitStatus1 = "fusermount: exit status 1"

// platformFuseChecks can get overridden by arch-specific files
// to run fuse checks (like checking the OSXFUSE version).
var platformFuseChecks = func(*core.IpfsNode) error {
	return nil
}

func Mount(node *core.IpfsNode, fsdir, nsdir, mfsdir string) error {
	// check if we already have live mounts.
	// if the user said "Mount", then there must be something wrong.
	// so, close them and try again.
	Unmount(node)

	if err := platformFuseChecks(node); err != nil {
		return err
	}

	return doMount(node, fsdir, nsdir, mfsdir)
}

func Unmount(node *core.IpfsNode) {
	if node.Mounts.Ipfs != nil && node.Mounts.Ipfs.IsActive() {
		// best effort
		if err := node.Mounts.Ipfs.Unmount(); err != nil {
			log.Errorf("error unmounting IPFS: %s", err)
		}
	}
	if node.Mounts.Ipns != nil && node.Mounts.Ipns.IsActive() {
		// best effort
		if err := node.Mounts.Ipns.Unmount(); err != nil {
			log.Errorf("error unmounting IPNS: %s", err)
		}
	}
	if node.Mounts.Mfs != nil && node.Mounts.Mfs.IsActive() {
		// best effort
		if err := node.Mounts.Mfs.Unmount(); err != nil {
			log.Errorf("error unmounting MFS: %s", err)
		}
	}
}

func doMount(node *core.IpfsNode, fsdir, nsdir, mfsdir string) error {
	fmtFuseErr := func(err error, mountpoint string) error {
		s := err.Error()
		if strings.Contains(s, fuseNoDirectory) {
			s = strings.Replace(s, `fusermount: "fusermount:`, "", -1)
			s = strings.Replace(s, `\n", exit status 1`, "", -1)
			return errors.New(s)
		}
		if s == fuseExitStatus1 {
			s = fmt.Sprintf("fuse failed to access mountpoint %s", mountpoint)
			return errors.New(s)
		}
		return err
	}

	// this sync stuff is so that both can be mounted simultaneously.
	var fsmount, nsmount, mfmount mount.Mount
	var err1, err2, err3 error

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		fsmount, err1 = rofs.Mount(node, fsdir)
	}()

	if node.IsOnline {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nsmount, err2 = ipns.Mount(node, nsdir, fsdir)
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		mfmount, err3 = mfs.Mount(node, mfsdir)
	}()

	wg.Wait()

	if err1 != nil {
		log.Errorf("error mounting IPFS %s: %s", fsdir, err1)
	}

	if err2 != nil {
		log.Errorf("error mounting IPNS %s for IPFS %s: %s", nsdir, fsdir, err2)
	}

	if err3 != nil {
		log.Errorf("error mounting MFS %s: %s", mfsdir, err3)
	}

	if err1 != nil || err2 != nil || err3 != nil {
		if fsmount != nil {
			_ = fsmount.Unmount()
		}
		if nsmount != nil {
			_ = nsmount.Unmount()
		}
		if mfmount != nil {
			_ = mfmount.Unmount()
		}

		if err1 != nil {
			return fmtFuseErr(err1, fsdir)
		}
		if err2 != nil {
			return fmtFuseErr(err2, nsdir)
		}
		return fmtFuseErr(err3, mfsdir)
	}

	// setup node state, so that it can be canceled
	node.Mounts.Ipfs = fsmount
	node.Mounts.Ipns = nsmount
	node.Mounts.Mfs = mfmount
	return nil
}
