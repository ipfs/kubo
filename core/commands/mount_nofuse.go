// +build (linux darwin freebsd) and nofuse

package commands

import (
	"errors"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
)

var MountCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Mounts IPFS to the filesystem (read-only) - !!Disabled!!",
		ShortDescription: `
Caution! Your version ipfs is compiled _without_ fuse support.
`,
	},
}

func Mount(node *core.IpfsNode, fsdir, nsdir string) error {
	return errors.New("not compiled in")
}
