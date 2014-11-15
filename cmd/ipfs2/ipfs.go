package main

import (
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	commands "github.com/jbenet/go-ipfs/core/commands2"
)

// This is the CLI root, used for executing commands accessible to CLI clients.
// Some subcommands (like 'ipfs daemon' or 'ipfs init') are only accessible here,
// and can't be called through the HTTP API.
var Root = &cmds.Command{
	Options:  commands.Root.Options,
	Helptext: commands.Root.Helptext,
}

// commandsClientCmd is the "ipfs commands" command for local cli
var commandsClientCmd = commands.CommandsCmd(Root)

// Commands in localCommands should always be run locally (even if daemon is running).
// They can override subcommands in commands.Root by defining a subcommand with the same name.
var localCommands = map[string]*cmds.Command{
	"daemon":   daemonCmd,
	"init":     initCmd,
	"tour":     tourCmd,
	"commands": commandsClientCmd,
}
var localMap = make(map[*cmds.Command]bool)

func init() {
	// setting here instead of in literal to prevent initialization loop
	// (some commands make references to Root)
	Root.Subcommands = localCommands

	// copy all subcommands from commands.Root into this root (if they aren't already present)
	for k, v := range commands.Root.Subcommands {
		if _, found := Root.Subcommands[k]; !found {
			Root.Subcommands[k] = v
		}
	}

	for _, v := range localCommands {
		localMap[v] = true
	}
}

// isLocal returns true if the command should only be run locally (not sent to daemon), otherwise false
func isLocal(cmd *cmds.Command) bool {
	_, found := localMap[cmd]
	return found
}

type cmdDetails struct {
	cannotRunOnClient bool
	cannotRunOnDaemon bool
	doesNotUseRepo    bool

	// initializesConfig describes commands that initialize the config.
	// pre-command hooks that require configs must not be run before this
	// command
	initializesConfig bool

	preemptsAutoUpdate bool
}

func (d *cmdDetails) String() string {
	return fmt.Sprintf("on client? %t, on daemon? %t, uses repo? %t",
		d.canRunOnClient(), d.canRunOnDaemon(), d.usesRepo())
}

func (d *cmdDetails) canRunOnClient() bool { return !d.cannotRunOnClient }
func (d *cmdDetails) canRunOnDaemon() bool { return !d.cannotRunOnDaemon }
func (d *cmdDetails) usesRepo() bool       { return !d.doesNotUseRepo }

// "What is this madness!?" you ask. Our commands have the unfortunate problem of
// not being able to run on all the same contexts. This map describes these
// properties so that other code can make decisions about whether to invoke a
// command or return an error to the user.
var cmdDetailsMap = map[*cmds.Command]cmdDetails{
	initCmd:                    cmdDetails{initializesConfig: true, cannotRunOnDaemon: true, doesNotUseRepo: true},
	daemonCmd:                  cmdDetails{cannotRunOnDaemon: true},
	commandsClientCmd:          cmdDetails{doesNotUseRepo: true},
	commands.CommandsDaemonCmd: cmdDetails{doesNotUseRepo: true},
	commands.DiagCmd:           cmdDetails{cannotRunOnClient: true},
	commands.VersionCmd:        cmdDetails{doesNotUseRepo: true},
	commands.UpdateCmd:         cmdDetails{preemptsAutoUpdate: true, cannotRunOnDaemon: true},
	commands.UpdateCheckCmd:    cmdDetails{preemptsAutoUpdate: true},
	commands.UpdateLogCmd:      cmdDetails{preemptsAutoUpdate: true},
	commands.LogCmd:            cmdDetails{cannotRunOnClient: true},
}
