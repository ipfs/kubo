// +build linux darwin freebsd netbsd openbsd
// +build nofuse

package commands

import (
	cmds "github.com/ipfs/go-ipfs/commands"

	"gx/ipfs/QmVD1W3MC8Hk1WZgFQPWWmBECJ3X72BgUYf9eCQ4PGzPps/go-ipfs-cmdkit"
)

var MountCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Mounts ipfs to the filesystem (disabled).",
		ShortDescription: `
This version of ipfs is compiled without fuse support, which is required
for mounting. If you'd like to be able to mount, please use a version of
ipfs compiled with fuse.

For the latest instructions, please check the project's repository:
  http://github.com/ipfs/go-ipfs
`,
	},
}
