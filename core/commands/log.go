package commands

import (
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"
)

const (
	// allLogSubsystems is used to specify all log subsystems when setting the
	// log level.
	allLogSubsystems = "all"
	// defaultLogLevel is used to request and to identify the default log
	// level.
	defaultLogLevel = "default"
)

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

This provides a dynamic, runtime alternative to the GOLOG_LOG_LEVEL
environment variable documented in 'ipfs log'.
`,
	},

	Arguments: []cmds.Argument{
		// TODO use a different keyword for 'all' because all can theoretically
		// clash with a subsystem name
		cmds.StringArg("subsystem", true, false, fmt.Sprintf("The subsystem logging identifier. Use '%s' to set all subsystems and the default level.", allLogSubsystems)),
		cmds.StringArg("level", true, false, fmt.Sprintf("The log level, with 'debug' as the most verbose and 'fatal' the least verbose. Use '%s' to set to the current default level.\n     One of: debug, info, warn, error, dpanic, panic, fatal, %s", defaultLogLevel, defaultLogLevel)),
	},
	NoLocal: true,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		args := req.Arguments
		subsystem, level := args[0], args[1]

		if subsystem == allLogSubsystems || subsystem == "*" {
			subsystem = "*"
		}

		if level == defaultLogLevel {
			level = logging.DefaultLevel().String()
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
			logLevel, err := logging.Parse(logLevelString)
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
		cmds.StringArg("subsystem", false, false, fmt.Sprintf("The subsystem logging identifier. Use '%s' for the default level. If not specified, returns levels for all subsystems.", defaultLogLevel)),
	},
	NoLocal: true,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		var subsystem string
		if len(req.Arguments) > 0 {
			subsystem = req.Arguments[0]
		}

		switch subsystem {
		case "":
			// Return levels for all subsystems (default behavior)
			levels := logging.SubsystemLevelNames()

			// Replace the log package default name with the command default name.
			delete(levels, logging.DefaultName)
			levels[defaultLogLevel] = logging.DefaultLevel().String()
			return cmds.EmitOnce(res, &logLevelsOutput{Levels: levels})
		case defaultLogLevel:
			// Return the default log level
			levelMap := map[string]string{defaultLogLevel: logging.DefaultLevel().String()}
			return cmds.EmitOnce(res, &logLevelsOutput{Levels: levelMap})
		default:
			// Return level for a specific subsystem.
			level, err := logging.SubsystemLevelName(subsystem)
			if err != nil {
				return err
			}
			levelMap := map[string]string{subsystem: level}
			return cmds.EmitOnce(res, &logLevelsOutput{Levels: levelMap})
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *logLevelsOutput) error {
			// Check if this is an RPC call by looking for the encoding option
			encoding, _ := req.Options["encoding"].(string)
			isRPC := encoding == "json"

			// If there are multiple subsystems (no specific subsystem requested), always show names
			showNames := isRPC || len(out.Levels) > 1

			for subsystem, level := range out.Levels {
				if showNames {
					// Show subsystem name when it's RPC or when showing multiple subsystems
					fmt.Fprintf(w, "%s: %s\n", subsystem, level)
				} else {
					// For CLI calls with single subsystem, only show the level
					fmt.Fprintf(w, "%s\n", level)
				}
			}
			return nil
		}),
	},
	Type: logLevelsOutput{},
}

type logLevelsOutput struct {
	Levels map[string]string
}
