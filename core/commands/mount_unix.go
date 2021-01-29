// +build !windows,!nofuse

package commands

import (
	"fmt"
	"io"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	nodeMount "github.com/ipfs/go-ipfs/fuse/node"

	cmds "github.com/ipfs/go-ipfs-cmds"
	config "github.com/ipfs/go-ipfs-config"
)

const (
	mountIPFSPathOptionName = "ipfs-path"
	mountIPNSPathOptionName = "ipns-path"
)

var MountCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Mounts IPFS to the filesystem (read-only).",
		ShortDescription: `
Mount IPFS at a read-only mountpoint on the OS (default: /ipfs and /ipns).
All IPFS objects will be accessible under that directory. Note that the
root will not be listable, as it is virtual. Access known paths directly.

You may have to create /ipfs and /ipns before using 'ipfs mount':

> sudo mkdir /ipfs /ipns
> sudo chown $(whoami) /ipfs /ipns
> ipfs daemon &
> ipfs mount
`,
		LongDescription: `
Mount IPFS at a read-only mountpoint on the OS. The default, /ipfs and /ipns,
are set in the configuration file, but can be overridden by the options.
All IPFS objects will be accessible under this directory. Note that the
root will not be listable, as it is virtual. Access known paths directly.

You may have to create /ipfs and /ipns before using 'ipfs mount':

> sudo mkdir /ipfs /ipns
> sudo chown $(whoami) /ipfs /ipns
> ipfs daemon &
> ipfs mount

Example:

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
		cmds.StringOption(mountIPFSPathOptionName, "f", "The path where IPFS should be mounted."),
		cmds.StringOption(mountIPNSPathOptionName, "n", "The path where IPNS should be mounted."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cfg, err := cmdenv.GetConfig(env)
		if err != nil {
			return err
		}

		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		// error if we aren't running node in online mode
		if !nd.IsOnline {
			return ErrNotOnline
		}

		fsdir, found := req.Options[mountIPFSPathOptionName].(string)
		if !found {
			fsdir = cfg.Mounts.IPFS // use default value
		}

		// get default mount points
		nsdir, found := req.Options[mountIPNSPathOptionName].(string)
		if !found {
			nsdir = cfg.Mounts.IPNS // NB: be sure to not redeclare!
		}

		err = nodeMount.Mount(nd, fsdir, nsdir)
		if err != nil {
			return err
		}

		var output config.Mounts
		output.IPFS = fsdir
		output.IPNS = nsdir
		return cmds.EmitOnce(res, &output)
	},
	Type: config.Mounts{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, mounts *config.Mounts) error {
			fmt.Fprintf(w, "IPFS mounted at: %s\n", cmdenv.EscNonPrint(mounts.IPFS))
			fmt.Fprintf(w, "IPNS mounted at: %s\n", cmdenv.EscNonPrint(mounts.IPNS))

			return nil
		}),
	},
}
