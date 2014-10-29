package cli

import (
	"fmt"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
)

// Parse parses the input commandline string (cmd, flags, and args).
// returns the corresponding command Request object.
func Parse(input []string, root *cmds.Command) (cmds.Request, error) {
	path, input, cmd := parsePath(input, root)
	opts, args, err := parseOptions(input)
	if err != nil {
		return nil, err
	}

	// TODO: figure out how to know when to read given file(s) as an input stream
	// (instead of filename arg string)

	return cmds.NewRequest(path, opts, args, nil, cmd), nil
}

// parsePath gets the command path from the command line input
func parsePath(input []string, root *cmds.Command) ([]string, []string, *cmds.Command) {
	cmd := root
	i := 0

	for _, blob := range input {
		if strings.HasPrefix(blob, "-") {
			break
		}

		sub := cmd.Subcommand(blob)
		if sub == nil {
			break
		}
		cmd = sub

		i++
	}

	return input[:i], input[i:], cmd
}

// parseOptions parses the raw string values of the given options
// returns the parsed options as strings, along with the CLI args
func parseOptions(input []string) (map[string]interface{}, []string, error) {
	opts := make(map[string]interface{})
	args := []string{}

	for i := 0; i < len(input); i++ {
		blob := input[i]

		if strings.HasPrefix(blob, "-") {
			name := blob[1:]
			value := ""

			// support single and double dash
			if strings.HasPrefix(name, "-") {
				name = name[1:]
			}

			if strings.Contains(name, "=") {
				split := strings.SplitN(name, "=", 2)
				name = split[0]
				value = split[1]
			}

			if _, ok := opts[name]; ok {
				return nil, nil, fmt.Errorf("Duplicate values for option '%s'", name)
			}

			opts[name] = value

		} else {
			args = append(args, blob)
		}
	}

	return opts, args, nil
}
