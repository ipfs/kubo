package commands

import (
	"fmt"

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
		Description: "List all available commands.",
		Help: `Lists all available commands (and subcommands) and exits.
	`,

		Run: func(req cmds.Request) (interface{}, error) {
			root := outputCommand("ipfs", root)
			return &root, nil
		},
		Marshallers: map[cmds.EncodingType]cmds.Marshaller{
			cmds.Text: func(res cmds.Response) ([]byte, error) {
				v := res.Output().(*Command)
				s := formatCommand("", v)
				return []byte(s), nil
			},
		},
		Type: &Command{},
	}
}

func outputCommand(name string, cmd *cmds.Command) Command {
	output := Command{
		Name:        name,
		Subcommands: make([]Command, len(cmd.Subcommands)),
	}

	i := 0
	for name, sub := range cmd.Subcommands {
		output.Subcommands[i] = outputCommand(name, sub)
		i++
	}

	return output
}

func formatCommand(prefix string, cmd *Command) string {
	if len(prefix) > 0 {
		prefix += " "
	}
	s := fmt.Sprintf("%s%s\n", prefix, cmd.Name)

	prefix += cmd.Name
	for _, sub := range cmd.Subcommands {
		s += formatCommand(prefix, &sub)
	}

	return s
}
