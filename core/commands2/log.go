package commands

import (
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	u "github.com/jbenet/go-ipfs/util"
)

var logCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.Argument{"subsystem", cmds.ArgString, true, false},
		cmds.Argument{"level", cmds.ArgString, true, false},
	},
	Help: "TODO",
	Run: func(res cmds.Response, req cmds.Request) {
		args := req.Arguments()
		if err := u.SetLogLevel(args[0].(string), args[1].(string)); err != nil {
			res.SetError(err, cmds.ErrClient)
			return
		}

		s := fmt.Sprintf("Changed log level of '%s' to '%s'", args[0], args[1])
		res.SetOutput(&MessageOutput{s})
	},
	Format: MessageMarshaller,
	Type:   &MessageOutput{},
}
