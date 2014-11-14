package commands

import (
	"errors"

	cmds "github.com/jbenet/go-ipfs/commands"
)

var ipfsMount = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Not yet implemented on Windows",
		ShortDescription: "Not yet implemented on Windows. :(",
	},

	Run: func(req cmds.Request) (interface{}, error) {
		return errors.New("Mount isn't compatible with Windows yet"), nil
	},
}
