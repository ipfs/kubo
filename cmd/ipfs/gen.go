package main

import (
	"fmt"
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

// command is the descriptor of an ipfs daemon command.
// Used with makeCommand to proxy over commands via the daemon.
type command struct {
	name      string
	args      int
	flags     []string
	online    bool
	cmdFn     commands.CmdFunc
	argFilter func(string) (string, error)
}

// commanderFunc is a function that can be passed into the Commander library as
// a command handler. Defined here because commander lacks this definition.
type commanderFunc func(*commander.Command, []string) error

// makeCommand Wraps a commands.CmdFunc so that it may be safely run by the
// commander library
func makeCommand(cmdDesc command) commanderFunc {
	return func(c *commander.Command, inp []string) error {
		if len(inp) < cmdDesc.args {
			u.POut(c.Long)
			return nil
		}
		confdir, err := getConfigDir(c)
		if err != nil {
			return err
		}

		cmd := daemon.NewCommand()
		cmd.Command = cmdDesc.name
		if cmdDesc.argFilter != nil {
			for _, a := range inp {
				s, err := cmdDesc.argFilter(a)
				if err != nil {
					return err
				}
				cmd.Args = append(cmd.Args, s)
			}
		} else {
			cmd.Args = inp
		}

		for _, a := range cmdDesc.flags {
			cmd.Opts[a] = c.Flag.Lookup(a).Value.Get()
		}

		err = daemon.SendCommand(cmd, confdir)
		if err != nil {
			log.Info("Executing command locally: %s", err)
			// Do locally
			n, err := localNode(confdir, cmdDesc.online)
			if err != nil {
				return fmt.Errorf("Local node creation failed: %v", err)
			}

			return cmdDesc.cmdFn(n, cmd.Args, cmd.Opts, os.Stdout)
		}
		return nil
	}
}
