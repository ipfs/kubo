package commands

import (
	"errors"

	cmds "github.com/jbenet/go-ipfs/commands"
)

var ipfsMount = &cmds.Command{
	Help: `Not yet implemented on Windows.`,
	Run: func(res cmds.Response, req cmds.Request) {
		res.SetError(errors.New("Mount isn't compatible with Windows yet"), cmds.ErrNormal)
	},
}
