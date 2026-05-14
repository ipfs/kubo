// Stub for non-FUSE builds: the complement of mount_unix.go's
// (linux || darwin || freebsd) && !nofuse, excluding windows
// which has its own stub in mount_windows.go.
//go:build !windows && (nofuse || !(linux || darwin || freebsd))

package commands

import (
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var MountCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Mounts ipfs to the filesystem (disabled).",
		ShortDescription: `
This version of ipfs is compiled without fuse support, which is required
for mounting. If you'd like to be able to mount, please use a version of
Kubo compiled with fuse.

For the latest instructions, please check the project's repository:
  http://github.com/ipfs/kubo
  https://github.com/ipfs/kubo/blob/master/docs/fuse.md
`,
	},
}
