package commands

import (
	"errors"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
)

var mountCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Not yet implemented on Windows",
		ShortDescription: "Not yet implemented on Windows. :(",
	},

	Run: func(req cmds.Request) (interface{}, error) {
		return errors.New("Mount isn't compatible with Windows yet"), nil
	},
}

func Mount(node *core.IpfsNode, fsdir, nsdir string) error {
	// TODO
	// currently a no-op, but we don't want to return an error
	return nil
}
