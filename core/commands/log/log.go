package commands

import (
	"fmt"
	"io"
	"strings"

	cmds "github.com/jbenet/go-ipfs/thirdparty/commands"
	ccutil "github.com/jbenet/go-ipfs/core/commands/util"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	u "github.com/jbenet/go-ipfs/util"

	tail "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/ActiveState/tail"
)

var log = eventlog.Logger("core/cmds/log")

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
		cmds.StringArg("level", true, false, "one of: debug, info, notice, warning, error, critical"),
	},
	Run: func(req cmds.Request, res cmds.Response) {

		args := req.Arguments()
		subsystem, level := args[0], args[1]

		if subsystem == logAllKeyword {
			subsystem = "*"
		}

		if err := u.SetLogLevel(subsystem, level); err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		s := fmt.Sprintf("Changed log level of '%s' to '%s'", subsystem, level)
		log.Info(s)
		res.SetOutput(&ccutil.MessageOutput{s})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: ccutil.MessageTextMarshaler,
	},
	Type: ccutil.MessageOutput{},
}

var logTailCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Read the logs",
		ShortDescription: `
'ipfs log tail' is a utility command used to read log output as it is written.
`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
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
			defer t.Stop()

			done := req.Context().Context.Done()

			for line := range t.Lines {
				// return when context closes
				select {
				case <-done:
					return
				default:
				}

				if line.Err != nil {
					fmt.Println(err.Error())
					return
				}
				// TODO: unpack the line text into a struct and output that
				outChan <- &ccutil.MessageOutput{line.Text}
			}
		}()

		res.SetOutput((<-chan interface{})(outChan))
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
					output := v.(*ccutil.MessageOutput)
					return strings.NewReader(output.Message + "\n"), nil
				},
			}, nil
		},
	},
	Type: ccutil.MessageOutput{},
}
