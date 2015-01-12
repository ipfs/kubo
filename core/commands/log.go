package commands

import (
	"fmt"
	"io"
	"strings"

	tail "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/ActiveState/tail"
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
		Tagline: "Interact with the daemon log output",
		ShortDescription: `
'ipfs log' contains utility commands to affect or read the logging
output of a running daemon.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"level": logLevelCmd,
		"read":  logReadCmd,
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

var logReadCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Read the logs",
		ShortDescription: `
'ipfs log read' is a utility command used to read the log output.

By default, the last 50 lines are returned. For more, use '-n=<number of lines>'.
Use '--stream' (or '-s') to stream the current output (in addition to the '-n' lines).
`,
	},

	Run: func(req cmds.Request) (interface{}, error) {
		path := fmt.Sprintf("%s/logs/events.log", req.Context().ConfigRoot)

		outChan := make(chan interface{})

		go func() {
			defer close(outChan)

			t, err := tail.TailFile(path, tail.Config{
				Location:  &tail.SeekInfo{0, 2},
				Follow:    true,
				MustExist: true,
				Logger:    tail.DiscardingLogger,
			})
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			for line := range t.Lines {
				if line.Err != nil {
					fmt.Println(err.Error())
					return
				}
				// TODO: unpack the line text into a struct and output that
				outChan <- &MessageOutput{line.Text}
			}
		}()

		return (<-chan interface{})(outChan), nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(<-chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			return &cmds.ChannelMarshaler{
				Channel: outChan,
				Marshaler: func(v interface{}) (io.Reader, error) {
					output := v.(*MessageOutput)
					return strings.NewReader(output.Message + "\n"), nil
				},
			}, nil
		},
	},
	Type: MessageOutput{},
}
