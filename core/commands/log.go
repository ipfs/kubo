package commands

import (
	"fmt"
	"io"
	"slices"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"
)

const (
	// allLogSubsystems is used to specify all log subsystems when setting the
	// log level.
	allLogSubsystems = "*"
	// allLogSubsystemsAlias is a convenience alias for allLogSubsystems that
	// doesn't require shell escaping.
	allLogSubsystemsAlias = "all"
	// defaultLogLevel is used to request and to identify the default log
	// level.
	defaultLogLevel = "default"
	// defaultSubsystemKey is the subsystem name that is used to denote the
	// default log level. We use parentheses for UI clarity to distinguish it
	// from regular subsystem names.
	defaultSubsystemKey = "(default)"
	// logLevelOption is an option for the tail subcommand to select the log
	// level to output.
	logLevelOption = "log-level"
	// noSubsystemSpecified is used when no subsystem argument is provided
	noSubsystemSpecified = ""
)

type logLevelOutput struct {
	Levels  map[string]string `json:",omitempty"`
	Message string            `json:",omitempty"`
}

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
		"level": logLevelCmd,
		"ls":    logLsCmd,
		"tail":  logTailCmd,
	},
}

var logLevelCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Change or get the logging level.",
		ShortDescription: `
Get or change the logging level of one or all logging subsystems.

This command provides a runtime alternative to the GOLOG_LOG_LEVEL
environment variable for debugging and troubleshooting.

UNDERSTANDING DEFAULT vs '*':

The "default" level is the fallback used by unconfigured subsystems.
You cannot set the default level directly - it only changes when you use '*'.

The '*' wildcard represents ALL subsystems including the default level.
Setting '*' changes everything at once, including the default.

EXAMPLES - Getting levels:

  ipfs log level              # Show only the default fallback level
  ipfs log level all          # Show all subsystem levels (100+ lines)
  ipfs log level core         # Show level for 'core' subsystem only

EXAMPLES - Setting levels:

  ipfs log level core debug   # Set 'core' to 'debug' (default unchanged)
  ipfs log level all info     # Set ALL to 'info' (including default)
  ipfs log level core default # Reset 'core' to use current default level

WILDCARD OPTIONS:

Use 'all' (convenient) or '*' (requires escaping) to affect all subsystems:
  ipfs log level all debug    # Convenient - no shell escaping needed
  ipfs log level '*' debug    # Equivalent but needs quotes: '*' or "*" or \*

BEHAVIOR EXAMPLES:

Initial state (all using default 'error'):
  $ ipfs log level              => error
  $ ipfs log level core         => error

After setting one subsystem:
  $ ipfs log level core debug
  $ ipfs log level              => error (default unchanged!)
  $ ipfs log level core         => debug (explicitly set)
  $ ipfs log level dht          => error (still uses default)

After setting everything with 'all':
  $ ipfs log level all info
  $ ipfs log level              => info (default changed!)
  $ ipfs log level core         => info (all changed)
  $ ipfs log level dht          => info (all changed)

The 'default' keyword always refers to the current default level:
  $ ipfs log level              => error
  $ ipfs log level core default  # Sets core to 'error'
  $ ipfs log level all info      # Changes default to 'info'
  $ ipfs log level core default  # Now sets core to 'info'
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("subsystem", false, false, fmt.Sprintf("The subsystem logging identifier. Use '%s' or '%s' to get or set the log level of all subsystems including the default. If not specified, only show the default log level.", allLogSubsystemsAlias, allLogSubsystems)),
		cmds.StringArg("level", false, false, fmt.Sprintf("The log level, with 'debug' as the most verbose and 'fatal' the least verbose. Use '%s' to set to the current default level. One of: debug, info, warn, error, dpanic, panic, fatal, %s", defaultLogLevel, defaultLogLevel)),
	},
	NoLocal: true,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		var level, subsystem string

		if len(req.Arguments) > 0 {
			subsystem = req.Arguments[0]
			if len(req.Arguments) > 1 {
				level = req.Arguments[1]
			}

			// Normalize aliases to the canonical "*" form
			if subsystem == allLogSubsystems || subsystem == allLogSubsystemsAlias {
				subsystem = "*"
			}
		}

		// If a level is specified, then set the log level.
		if level != "" {
			if level == defaultLogLevel {
				level = logging.DefaultLevel().String()
			}

			if err := logging.SetLogLevel(subsystem, level); err != nil {
				return err
			}

			s := fmt.Sprintf("Changed log level of '%s' to '%s'\n", subsystem, level)
			log.Info(s)

			return cmds.EmitOnce(res, &logLevelOutput{Message: s})
		}

		// Get the level for the requested subsystem.
		switch subsystem {
		case noSubsystemSpecified:
			// Return the default log level
			levelMap := map[string]string{logging.DefaultName: logging.DefaultLevel().String()}
			return cmds.EmitOnce(res, &logLevelOutput{Levels: levelMap})
		case allLogSubsystems, allLogSubsystemsAlias:
			// Return levels for all subsystems (default behavior)
			levels := logging.SubsystemLevelNames()

			// Replace default subsystem key with defaultSubsystemKey.
			levels[defaultSubsystemKey] = levels[logging.DefaultName]
			delete(levels, logging.DefaultName)
			return cmds.EmitOnce(res, &logLevelOutput{Levels: levels})
		default:
			// Return level for a specific subsystem.
			level, err := logging.SubsystemLevelName(subsystem)
			if err != nil {
				return err
			}
			levelMap := map[string]string{subsystem: level}
			return cmds.EmitOnce(res, &logLevelOutput{Levels: levelMap})
		}

	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *logLevelOutput) error {
			if out.Message != "" {
				fmt.Fprint(w, out.Message)
				return nil
			}

			// Check if this is an RPC call by looking for the encoding option
			encoding, _ := req.Options["encoding"].(string)
			isRPC := encoding == "json"

			// Determine whether to show subsystem names in output.
			// Show subsystem names when:
			// 1. It's an RPC call (needs JSON structure with named fields)
			// 2. Multiple subsystems are displayed (for clarity when showing many levels)
			showNames := isRPC || len(out.Levels) > 1

			levelNames := make([]string, 0, len(out.Levels))
			for subsystem, level := range out.Levels {
				if showNames {
					// Show subsystem name when it's RPC or when showing multiple subsystems
					levelNames = append(levelNames, fmt.Sprintf("%s: %s", subsystem, level))
				} else {
					// For CLI calls with single subsystem, only show the level
					levelNames = append(levelNames, level)
				}
			}
			slices.Sort(levelNames)
			for _, ln := range levelNames {
				fmt.Fprintln(w, ln)
			}
			return nil
		}),
	},
	Type: logLevelOutput{},
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
