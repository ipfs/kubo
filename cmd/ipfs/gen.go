package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/config"
	"github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

// CommanderFunc is a function that can be passed into the Commander library as
// a command handler. Defined here because commander lacks this definition.
type CommanderFunc func(*commander.Command, []string) error

// MakeCommand Wraps a commands.CmdFunc so that it may be safely run by the
// commander library
func MakeCommand(cmdName string, expargs []string, cmdFn commands.CmdFunc) CommanderFunc {
	return func(c *commander.Command, inp []string) error {
		if len(inp) < 1 {
			u.POut(c.Long)
			return nil
		}
		confdir, err := getConfigDir(c.Parent)
		if err != nil {
			return err
		}

		confapi, err := config.ReadConfigKey(confdir+"/config", "Addresses.API")
		if err != nil {
			return err
		}

		apiaddr, ok := confapi.(string)
		if !ok {
			return errors.New("ApiAddress in config file was not a string")
		}

		cmd := daemon.NewCommand()
		cmd.Command = cmdName
		cmd.Args = inp

		for _, a := range expargs {
			cmd.Opts[a] = c.Flag.Lookup(a).Value.Get()
		}
		err = daemon.SendCommand(cmd, apiaddr)
		if err != nil {
			fmt.Printf("Executing command locally: %s", err)
			// Do locally
			n, err := localNode(confdir, false)
			if err != nil {
				return err
			}

			return cmdFn(n, cmd.Args, cmd.Opts, os.Stdout)
		}
		return nil
	}
}
