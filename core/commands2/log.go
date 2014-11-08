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
	// TODO UsageLine: "log <name> <level> ",
	// TODO Short:     "switch logging levels of a running daemon",
	Help: `ipfs log <subsystem> <level> - switch logging levels of a running daemon

   <subsystem> is a the subsystem logging identifier. Use * for all subsystems.
   <level> is one of: debug, info, notice, warning, error, critical

ipfs log is a utility command used to change the logging output of a running daemon.
`,
	Run: func(res cmds.Response, req cmds.Request) {
		args := req.Arguments()
		if err := u.SetLogLevel(args[0].(string), args[1].(string)); err != nil {
			res.SetError(err, cmds.ErrClient)
			return
		}

		s := fmt.Sprintf("Changed log level of '%s' to '%s'", args[0], args[1])
		res.SetOutput(&MessageOutput{s})
	},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: MessageTextMarshaller,
	},
	Type: &MessageOutput{},
}
