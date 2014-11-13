package main

import (
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

// Commands in localCommands should always be run locally (even if daemon is running).
// They can override subcommands in commands.Root by defining a subcommand with the same name.
var localCommands = map[string]*cmds.Command{
	"daemon":   daemonCmd, // TODO name
	"init":     initCmd,   // TODO name
	"tour":     cmdTour,
	"commands": commands.CommandsCmd(Root),
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
