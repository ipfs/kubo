package commands

import (
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	u "github.com/jbenet/go-ipfs/util"
)

var logCmd = &cmds.Command{
	Description: "Change the logging level",
	Help: `'ipfs log' is a utility command used to change the logging
output of a running daemon.
`,

	Arguments: []cmds.Argument{
		cmds.StringArg("subsystem", true, false, "the subsystem logging identifier. Use * for all subsystems."),
		cmds.StringArg("level", true, false, "one of: debug, info, notice, warning, error, critical"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		args := req.Arguments()
		if err := u.SetLogLevel(args[0].(string), args[1].(string)); err != nil {
			return nil, err
		}

		s := fmt.Sprintf("Changed log level of '%s' to '%s'", args[0], args[1])
		log.Info(s)
		return &MessageOutput{s}, nil
	},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: MessageTextMarshaller,
	},
	Type: &MessageOutput{},
}
