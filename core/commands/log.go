package commands

import (
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	logging "github.com/ipfs/go-ipfs/vendor/go-log-v1.0.0"
)

// Golang os.Args overrides * and replaces the character argument with
// an array which includes every file in the user's CWD. As a
// workaround, we use 'all' instead. The util library still uses * so
// we convert it at this step.
var logAllKeyword = "all"

var LogCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with the daemon log output",
		ShortDescription: `
'ipfs log' contains utility commands to affect or read the logging
output of a running daemon.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"level": logLevelCmd,
		"tail":  logTailCmd,
	},
}

var logLevelCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Change the logging level",
		ShortDescription: `
'ipfs log level' is a utility command used to change the logging
output of a running daemon.
`,
	},

	Arguments: []cmds.Argument{
		// TODO use a different keyword for 'all' because all can theoretically
		// clash with a subsystem name
		cmds.StringArg("subsystem", true, false, fmt.Sprintf("the subsystem logging identifier. Use '%s' for all subsystems.", logAllKeyword)),
		cmds.StringArg("level", true, false, "one of: debug, info, warning, error, fatal, panic"),
	},
	Run: func(req cmds.Request, res cmds.Response) {

		args := req.Arguments()
		subsystem, level := args[0], args[1]

		if subsystem == logAllKeyword {
			subsystem = "*"
		}

		if err := logging.SetLogLevel(subsystem, level); err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		s := fmt.Sprintf("Changed log level of '%s' to '%s'\n", subsystem, level)
		log.Info(s)
		res.SetOutput(&MessageOutput{s})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: MessageTextMarshaler,
	},
	Type: MessageOutput{},
}

var logTailCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Read the logs",
		ShortDescription: `
'ipfs log tail' is a utility command used to read log output as it is written.
`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		r, w := io.Pipe()
		logging.WriterGroup.AddWriter(w)
		go func() {
			<-req.Context().Done()
			w.Close()
		}()
		res.SetOutput(r)
	},
}
