/*
Package commands implements the IPFS command interface

Using github.com/ipfs/go-ipfs/commands to define the command line and
HTTP APIs.  This is the interface available to folks consuming IPFS
from outside of the Go language.
*/
package commands

import (
	"bytes"
	"io"
	"sort"

	cmds "github.com/ipfs/go-ipfs/commands"
)

type Command struct {
	Name        string
	Subcommands []Command
}

// CommandsCmd takes in a root command,
// and returns a command that lists the subcommands in that root
func CommandsCmd(root *cmds.Command) *cmds.Command {
	return &cmds.Command{
		Helptext: cmds.HelpText{
			Tagline:          "List all available commands.",
			ShortDescription: `Lists all available commands (and subcommands) and exits.`,
		},

		Run: func(req cmds.Request, res cmds.Response) {
			root := cmd2outputCmd("ipfs", root)
			res.SetOutput(&root)
		},
		Marshalers: cmds.MarshalerMap{
			cmds.Text: func(res cmds.Response) (io.Reader, error) {
				v := res.Output().(*Command)
				buf := new(bytes.Buffer)
				for _, s := range cmdPathStrings(v) {
					buf.Write([]byte(s + "\n"))
				}
				return buf, nil
			},
		},
		Type: Command{},
	}
}

func cmd2outputCmd(name string, cmd *cmds.Command) Command {
	output := Command{
		Name:        name,
		Subcommands: make([]Command, len(cmd.Subcommands)),
	}

	i := 0
	for name, sub := range cmd.Subcommands {
		output.Subcommands[i] = cmd2outputCmd(name, sub)
		i++
	}

	return output
}

func cmdPathStrings(cmd *Command) []string {
	var cmds []string

	var recurse func(prefix string, cmd *Command)
	recurse = func(prefix string, cmd *Command) {
		cmds = append(cmds, prefix+cmd.Name)
		for _, sub := range cmd.Subcommands {
			recurse(prefix+cmd.Name+" ", &sub)
		}
	}

	recurse("", cmd)
	sort.Sort(sort.StringSlice(cmds))
	return cmds
}
