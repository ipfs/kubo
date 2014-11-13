package main

import (
	cmds "github.com/jbenet/go-ipfs/commands"
	commands "github.com/jbenet/go-ipfs/core/commands2"
)

var Root = &cmds.Command{
	Options: commands.Root.Options,
	Help:    commands.Root.Help,
}

var rootSubcommands = map[string]*cmds.Command{
	"daemon":   daemonCmd, // TODO name
	"init":     initCmd,   // TODO name
	"tour":     cmdTour,
	"commands": commands.CommandsCmd(Root),
}

func init() {
	// setting here instead of in literal to prevent initialization loop
	// (some commands make references to Root)
	Root.Subcommands = rootSubcommands

	// copy all subcommands from commands.Root into this root (if they aren't already present)
	for k, v := range commands.Root.Subcommands {
		if _, found := Root.Subcommands[k]; !found {
			Root.Subcommands[k] = v
		}
	}
}
