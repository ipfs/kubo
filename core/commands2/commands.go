package commands

import (
	"fmt"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
)

type Command struct {
	Name        string
	Subcommands []Command
}

var commandsCmd = &cmds.Command{
	Description: "List all available commands.",
	Help: `Lists all available commands (and subcommands) and exits.
`,

	Run: func(req cmds.Request) (interface{}, error) {
		root := outputCommand("ipfs", Root)
		return &root, nil
	},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*Command)
			s := formatCommand(v, 0)
			return []byte(s), nil
		},
	},
	Type: &Command{},
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

func formatCommand(cmd *Command, depth int) string {
	var s string

	if depth > 0 {
		indent := strings.Repeat("    ", depth-1)
		s = fmt.Sprintf("%s%s\n", indent, cmd.Name)
	}

	for _, sub := range cmd.Subcommands {
		s += formatCommand(&sub, depth+1)
	}

	return s
}
