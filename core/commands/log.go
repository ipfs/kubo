package commands

import (
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"
)

// Golang os.Args overrides * and replaces the character argument with
// an array which includes every file in the user's CWD. As a
// workaround, we use 'all' instead. The util library still uses * so
// we convert it at this step.
var logAllKeyword = "all"

var LogCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with the daemon log output.",
		ShortDescription: `
'ipfs log' contains utility commands to affect or read the logging
output of a running daemon.

There are also two environmental variables that direct the logging
system (not just for the daemon logs, but all commands):
    GOLOG_LOG_LEVEL - sets the level of verbosity of the logging.
        One of: debug, info, warn, error, dpanic, panic, fatal
    GOLOG_LOG_FMT - sets formatting of the log output.
        One of: color, nocolor, json
`,
	},

	Subcommands: map[string]*cmds.Command{
		"level":     logLevelCmd,
		"get-level": logGetLevelCmd,
		"ls":        logLsCmd,
		"tail":      logTailCmd,
	},
}

var logLevelCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Change the logging level.",
		ShortDescription: `
Change the verbosity of one or all subsystems log output. This does not affect
the event log.

This provides a dynamic, runtime alternative to the GOLOG_LOG_LEVEL environment
variable documented in 'ipfs log'.
`,
	},

	Arguments: []cmds.Argument{
		// TODO use a different keyword for 'all' because all can theoretically
		// clash with a subsystem name
		cmds.StringArg("subsystem", true, false, fmt.Sprintf("The subsystem logging identifier. Use '%s' to set the global default level.", logAllKeyword)),
		cmds.StringArg("level", true, false, `The log level, with 'debug' the most verbose and 'fatal' the least verbose.
			One of: debug, info, warn, error, dpanic, panic, fatal.
		`),
	},
	NoLocal: true,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		args := req.Arguments
		subsystem, level := args[0], args[1]

		if subsystem == logAllKeyword {
			subsystem = "*"
		}

		if err := logging.SetLogLevel(subsystem, level); err != nil {
			return err
		}

		s := fmt.Sprintf("Changed log level of '%s' to '%s'\n", subsystem, level)
		log.Info(s)

		return cmds.EmitOnce(res, &MessageOutput{s})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *MessageOutput) error {
			fmt.Fprint(w, out.Message)
			return nil
		}),
	},
	Type: MessageOutput{},
}

var logLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List the logging subsystems.",
		ShortDescription: `
'ipfs log ls' is a utility command used to list the logging
subsystems of a running daemon.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		return cmds.EmitOnce(res, &stringList{logging.GetSubsystems()})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, list *stringList) error {
			for _, s := range list.Strings {
				fmt.Fprintln(w, s)
			}
			return nil
		}),
	},
	Type: stringList{},
}

const logLevelOption = "log-level"

var logTailCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Read and output log messages.",
		ShortDescription: `
Outputs log messages as they are generated.

NOTE: --log-level requires the server to be logging at least at this level

Example:

  GOLOG_LOG_LEVEL="error,bitswap=debug" ipfs daemon
  ipfs log tail --log-level info

This will only return 'info' logs from bitswap and skip 'debug'.
`,
	},

	Options: []cmds.Option{
		cmds.StringOption(logLevelOption, "Log level to listen to.").WithDefault(""),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		var pipeReader *logging.PipeReader
		logLevelString, _ := req.Options[logLevelOption].(string)
		if logLevelString != "" {
			logLevel, err := logging.LevelFromString(logLevelString)
			if err != nil {
				return fmt.Errorf("setting log level %s: %w", logLevelString, err)
			}
			pipeReader = logging.NewPipeReader(logging.PipeLevel(logLevel))
		} else {
			pipeReader = logging.NewPipeReader()
		}

		go func() {
			<-req.Context.Done()
			pipeReader.Close()
		}()
		return res.Emit(pipeReader)
	},
}

var logGetLevelCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get the logging level.",
		ShortDescription: `
'ipfs log get-level' is a utility command used to get the logging level for
a specific subsystem, the global default, or all subsystems.

This complements 'ipfs log level' and provides runtime visibility into the
current logging configuration (whether set by GOLOG_LOG_LEVEL environment
variable or changed dynamically).

Examples:
  ipfs log get-level       # Show levels for all subsystems
  ipfs log get-level all   # Show the global default level
  ipfs log get-level core  # Show the core subsystem level
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("subsystem", false, false, fmt.Sprintf("The subsystem logging identifier. Use '%s' for the global default level. If not specified, returns levels for all subsystems.", logAllKeyword)),
	},
	NoLocal: true,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		var subsystem string
		if len(req.Arguments) > 0 {
			subsystem = req.Arguments[0]
		}

		if subsystem == logAllKeyword || subsystem == "*" {
			// Return the global log level
			level, err := logging.GetLogLevel()
			if err != nil {
				return err
			}
			levelMap := map[string]string{"*": level}
			return cmds.EmitOnce(res, &logLevelsOutput{Levels: levelMap})
		} else if subsystem == "" {
			// Return levels for all subsystems (default behavior)
			levels := logging.GetAllLogLevels()
			// Also get the global level
			if globalLevel, err := logging.GetLogLevel(); err == nil {
				levels["*"] = globalLevel
			}
			return cmds.EmitOnce(res, &logLevelsOutput{Levels: levels})
		} else {
			// Return level for a specific subsystem
			level, err := logging.GetLogLevel(subsystem)
			if err != nil {
				return err
			}
			levelMap := map[string]string{subsystem: level}
			return cmds.EmitOnce(res, &logLevelsOutput{Levels: levelMap})
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *logLevelsOutput) error {
			for subsystem, level := range out.Levels {
				fmt.Fprintf(w, "%s: %s\n", subsystem, level)
			}
			return nil
		}),
	},
	Type: logLevelsOutput{},
}

type logLevelsOutput struct {
	Levels map[string]string
}
