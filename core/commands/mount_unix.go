// +build linux darwin freebsd

package commands

import (
	"fmt"
	"strings"
	"time"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
	core "github.com/jbenet/go-ipfs/core"
	ipns "github.com/jbenet/go-ipfs/fuse/ipns"
	mount "github.com/jbenet/go-ipfs/fuse/mount"
	rofs "github.com/jbenet/go-ipfs/fuse/readonly"
)

// amount of time to wait for mount errors
// TODO is this non-deterministic?
const mountTimeout = time.Second

// fuseNoDirectory used to check the returning fuse error
const fuseNoDirectory = "fusermount: failed to access mountpoint"

var MountCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Mounts IPFS to the filesystem (read-only)",
		Synopsis: `
ipfs mount [-f <ipfs mount path>] [-n <ipns mount path>]
`,
		ShortDescription: `
Mount ipfs at a read-only mountpoint on the OS (default: /ipfs and /ipns).
All ipfs objects will be accessible under that directory. Note that the
root will not be listable, as it is virtual. Access known paths directly.

You may have to create /ipfs and /ipfs before using 'ipfs mount':

> sudo mkdir /ipfs /ipns
> sudo chown ` + "`" + `whoami` + "`" + ` /ipfs /ipns
> ipfs daemon &
> ipfs mount
`,
		LongDescription: `
Mount ipfs at a read-only mountpoint on the OS (default: /ipfs and /ipns).
All ipfs objects will be accessible under that directory. Note that the
root will not be listable, as it is virtual. Access known paths directly.

You may have to create /ipfs and /ipfs before using 'ipfs mount':

> sudo mkdir /ipfs /ipns
> sudo chown ` + "`" + `whoami` + "`" + ` /ipfs /ipns
> ipfs daemon &
> ipfs mount

EXAMPLE:

# setup
> mkdir foo
> echo "baz" > foo/bar
> ipfs add -r foo
added QmWLdkp93sNxGRjnFHPaYg8tCQ35NBY3XPn6KiETd3Z4WR foo/bar
added QmSh5e7S6fdcu75LAbXNZAFY2nGyZUJXyLCJDvn2zRkWyC foo
> ipfs ls QmSh5e7S6fdcu75LAbXNZAFY2nGyZUJXyLCJDvn2zRkWyC
QmWLdkp93sNxGRjnFHPaYg8tCQ35NBY3XPn6KiETd3Z4WR 12 bar
> ipfs cat QmWLdkp93sNxGRjnFHPaYg8tCQ35NBY3XPn6KiETd3Z4WR
baz

# mount
> ipfs daemon &
> ipfs mount
IPFS mounted at: /ipfs
IPNS mounted at: /ipns
> cd /ipfs/QmSh5e7S6fdcu75LAbXNZAFY2nGyZUJXyLCJDvn2zRkWyC
> ls
bar
> cat bar
baz
> cat /ipfs/QmSh5e7S6fdcu75LAbXNZAFY2nGyZUJXyLCJDvn2zRkWyC/bar
baz
> cat /ipfs/QmWLdkp93sNxGRjnFHPaYg8tCQ35NBY3XPn6KiETd3Z4WR
baz
`,
	},

	Options: []cmds.Option{
		// TODO longform
		cmds.StringOption("f", "The path where IPFS should be mounted"),

		// TODO longform
		cmds.StringOption("n", "The path where IPNS should be mounted"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		cfg, err := req.Context().GetConfig()
		if err != nil {
			return nil, err
		}

		node, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		// error if we aren't running node in online mode
		if !node.OnlineMode() {
			return nil, errNotOnline
		}

		fsdir, found, err := req.Option("f").String()
		if err != nil {
			return nil, err
		}
		if !found {
			fsdir = cfg.Mounts.IPFS // use default value
		}

		// get default mount points
		nsdir, found, err := req.Option("n").String()
		if err != nil {
			return nil, err
		}
		if !found {
			nsdir = cfg.Mounts.IPNS // NB: be sure to not redeclare!
		}

		err = Mount(node, fsdir, nsdir)
		if err != nil {
			return nil, err
		}

		var output config.Mounts
		output.IPFS = fsdir
		output.IPNS = nsdir
		return &output, nil
	},
	Type: &config.Mounts{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*config.Mounts)
			s := fmt.Sprintf("IPFS mounted at: %s\n", v.IPFS)
			s += fmt.Sprintf("IPNS mounted at: %s\n", v.IPNS)
			return []byte(s), nil
		},
	},
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
		fsmount.Close()
		nsmount.Close()
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

var platformFuseChecks = func() error {
	return nil
}

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
