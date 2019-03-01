// +build !nofuse

package commands

import (
	"errors"
	"fmt"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	mount "github.com/ipfs/go-ipfs/core/commands/mount"
	"github.com/ipfs/go-ipfs/core/coreapi"

	cmds "gx/ipfs/QmX6AchyJgso1WNamTJMdxfzGiWuYu94K6tF9MJ66rRhAu/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var MountCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Mounts IPFS to the filesystem.",
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
are set in the configuration file, but can be overriden by the options.
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
	Options: []cmdkit.Option{
		cmdkit.StringOption("ipfs-path", "f", "The path where IPFS should be mounted."),
		//TODO: this should probably be an argument not an option; or possibly its own command `ipfs unmount [-f --timeout]`
		cmdkit.BoolOption("unmount", "u", "Destroy existing mount."),
	},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) (err error) {
		defer res.Close()

		daemon, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		destroy, _ := req.Options["unmount"].(bool)
		if destroy {
			if daemon.Mount == nil {
				return errors.New("IPFS is not mounted")
			}
			whence := daemon.Mount.Where()
			ret := daemon.Mount.Close()
			if ret == nil {
				cmds.EmitOnce(res, fmt.Sprintf("Successfully unmounted %#q", whence))
				daemon.Mount = nil
			}

			return ret
		}

		if daemon.Mount != nil {
			//TODO: introduce `mount -f` to automatically do this?
			//problem: '-f' overlaps with ipfs-path short param
			return fmt.Errorf("IPFS already mounted at: %#q use `ipfs mount -u`", daemon.Mount.Where())
		}

		api, err := coreapi.NewCoreAPI(daemon)
		if err != nil {
			return err
		}

		conf, err := cmdenv.GetConfig(env)
		if err != nil {
			return err
		}

		daemonCtx := daemon.Context()
		mountPoint := conf.Mounts.IPFS
		filesRoot := daemon.FilesRoot

		fsi, err := mount.InvokeMount(mountPoint, filesRoot, api, daemonCtx)
		if err != nil {
			return err
		}
		daemon.Mount = fsi
		cmds.EmitOnce(res, fmt.Sprintf("mounted at: %#q", daemon.Mount.Where()))
		return nil
	},
}
