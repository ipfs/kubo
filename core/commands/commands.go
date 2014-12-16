package commands

import (
	"bytes"
	"io"
	"sort"

	cmds "github.com/jbenet/go-ipfs/commands"
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

		Run: func(req cmds.Request) (interface{}, error) {
			root := cmd2outputCmd("ipfs", root)
			return &root, nil
		},
		Marshalers: cmds.MarshalerMap{
			cmds.Text: func(res cmds.Response) (io.Reader, error) {
				v := res.Output().(*Command)
				var buf bytes.Buffer
				for _, s := range cmdPathStrings(v) {
					buf.Write([]byte(s + "\n"))
				}
				return &buf, nil
			},
		},
		Type: &Command{},
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
