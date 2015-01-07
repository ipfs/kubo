package commands

import (
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	u "github.com/jbenet/go-ipfs/util"
)

// Golang os.Args overrides * and replaces the character argument with
// an array which includes every file in the user's CWD. As a
// workaround, we use 'all' instead. The util library still uses * so
// we convert it at this step.
var logAllKeyword = "all"

var LogCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Change the logging level",
		ShortDescription: `
'ipfs log' is a utility command used to change the logging
output of a running daemon.
`,
	},

	Arguments: []cmds.Argument{
		// TODO use a different keyword for 'all' because all can theoretically
		// clash with a subsystem name
		cmds.StringArg("subsystem", true, false, fmt.Sprintf("the subsystem logging identifier. Use '%s' for all subsystems.", logAllKeyword)),
		cmds.StringArg("level", true, false, "one of: debug, info, notice, warning, error, critical"),
	},
	Run: func(req cmds.Request) (interface{}, error) {

		args := req.Arguments()
		subsystem, level := args[0], args[1]

		if subsystem == logAllKeyword {
			subsystem = "*"
		}

		if err := u.SetLogLevel(subsystem, level); err != nil {
			return nil, err
		}

		s := fmt.Sprintf("Changed log level of '%s' to '%s'", subsystem, level)
		log.Info(s)
		return &MessageOutput{s}, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: MessageTextMarshaler,
	},
	Type: MessageOutput{},
}
