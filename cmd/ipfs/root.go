package main

import (
	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core/commands"
)

var Root = &cmds.Command{
	Options: commands.Root.Options,
	Help:    commands.Root.Help,
	Subcommands: map[string]*cmds.Command{
		"daemon": Daemon,
	},
}
