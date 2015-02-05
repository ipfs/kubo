package main

import (
	"fmt"

	cmds "github.com/jbenet/go-ipfs/thirdparty/commands"
	commands "github.com/jbenet/go-ipfs/core/commands"

	cmdcommands "github.com/jbenet/go-ipfs/core/commands/commands"
	daemon "github.com/jbenet/go-ipfs/core/commands/daemon"
	diag "github.com/jbenet/go-ipfs/core/commands/diag"
	cmdinit "github.com/jbenet/go-ipfs/core/commands/init"
	cmdlog "github.com/jbenet/go-ipfs/core/commands/log"
	tour "github.com/jbenet/go-ipfs/core/commands/tour"
	update "github.com/jbenet/go-ipfs/core/commands/update"
	version "github.com/jbenet/go-ipfs/core/commands/version"
)

// This is the CLI root, used for executing commands accessible to CLI clients.
// Some subcommands (like 'ipfs daemon' or 'ipfs init') are only accessible here,
// and can't be called through the HTTP API.
var Root = &cmds.Command{
	Options:  commands.Root.Options,
	Helptext: commands.Root.Helptext,
}

// commandsClientCmd is the "ipfs commands" command for local cli
var commandsClientCmd = cmdcommands.CommandsCmd(Root)

// Commands in localCommands should always be run locally (even if daemon is running).
// They can override subcommands in commands.Root by defining a subcommand with the same name.
var localCommands = map[string]*cmds.Command{
	"daemon":   daemon.DaemonCmd,
	"init":     cmdinit.InitCmd,
	"tour":     tour.TourCmd,
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

// NB: when necessary, properties are described using negatives in order to
// provide desirable defaults
type cmdDetails struct {
	cannotRunOnClient bool
	cannotRunOnDaemon bool
	doesNotUseRepo    bool

	// doesNotUseConfigAsInput describes commands that do not use the config as
	// input. These commands either initialize the config or perform operations
	// that don't require access to the config.
	//
	// pre-command hooks that require configs must not be run before these
	// commands.
	doesNotUseConfigAsInput bool

	// preemptsAutoUpdate describes commands that must be executed without the
	// auto-update pre-command hook
	preemptsAutoUpdate bool
}

func (d *cmdDetails) String() string {
	return fmt.Sprintf("on client? %t, on daemon? %t, uses repo? %t",
		d.canRunOnClient(), d.canRunOnDaemon(), d.usesRepo())
}

func (d *cmdDetails) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"canRunOnClient":     d.canRunOnClient(),
		"canRunOnDaemon":     d.canRunOnDaemon(),
		"preemptsAutoUpdate": d.preemptsAutoUpdate,
		"usesConfigAsInput":  d.usesConfigAsInput(),
		"usesRepo":           d.usesRepo(),
	}
}

func (d *cmdDetails) usesConfigAsInput() bool        { return !d.doesNotUseConfigAsInput }
func (d *cmdDetails) doesNotPreemptAutoUpdate() bool { return !d.preemptsAutoUpdate }
func (d *cmdDetails) canRunOnClient() bool           { return !d.cannotRunOnClient }
func (d *cmdDetails) canRunOnDaemon() bool           { return !d.cannotRunOnDaemon }
func (d *cmdDetails) usesRepo() bool                 { return !d.doesNotUseRepo }

// "What is this madness!?" you ask. Our commands have the unfortunate problem of
// not being able to run on all the same contexts. This map describes these
// properties so that other code can make decisions about whether to invoke a
// command or return an error to the user.
var cmdDetailsMap = map[*cmds.Command]cmdDetails{
	cmdinit.InitCmd: cmdDetails{doesNotUseConfigAsInput: true, cannotRunOnDaemon: true, doesNotUseRepo: true},

	// daemonCmd allows user to initialize the config. Thus, it may be called
	// without using the config as input
	daemon.DaemonCmd:           cmdDetails{doesNotUseConfigAsInput: true, cannotRunOnDaemon: true},
	commandsClientCmd:          cmdDetails{doesNotUseRepo: true},
	commands.CommandsDaemonCmd: cmdDetails{doesNotUseRepo: true},
	diag.DiagCmd:               cmdDetails{cannotRunOnClient: true},
	version.VersionCmd:         cmdDetails{doesNotUseConfigAsInput: true, doesNotUseRepo: true}, // must be permitted to run before init
	update.UpdateCmd:           cmdDetails{preemptsAutoUpdate: true, cannotRunOnDaemon: true},
	update.UpdateCheckCmd:      cmdDetails{preemptsAutoUpdate: true},
	update.UpdateLogCmd:        cmdDetails{preemptsAutoUpdate: true},
	cmdlog.LogCmd:              cmdDetails{cannotRunOnClient: true},
}
