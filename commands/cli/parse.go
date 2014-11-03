package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
)

// Parse parses the input commandline string (cmd, flags, and args).
// returns the corresponding command Request object.
func Parse(input []string, roots ...*cmds.Command) (cmds.Request, *cmds.Command, error) {
	var req cmds.Request
	var root *cmds.Command

	// use the root that matches the longest path (most accurately matches request)
	maxLength := 0
	for _, r := range roots {
		path, input, cmd := parsePath(input, r)
		opts, stringArgs, err := parseOptions(input)
		if err != nil {
			return nil, nil, err
		}

		length := len(path)
		if length > maxLength {
			maxLength = length

			args, err := parseArgs(stringArgs, cmd)
			if err != nil {
				return nil, nil, err
			}

			req = cmds.NewRequest(path, opts, args, cmd)
			root = r
		}
	}

	if maxLength == 0 {
		return nil, nil, errors.New("Not a valid subcommand")
	}

	return req, root, nil
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

// Note that the argument handling here is dumb, it does not do any error-checking.
// (Arguments are further processed when the request is passed to the command to run)
func parseArgs(stringArgs []string, cmd *cmds.Command) ([]interface{}, error) {
	args := make([]interface{}, len(cmd.Arguments))

	for i, arg := range cmd.Arguments {
		// TODO: handle variadic args
		if i >= len(stringArgs) {
			break
		}

		if arg.Type == cmds.ArgString {
			args[i] = stringArgs[i]

		} else {
			in, err := os.Open(stringArgs[i])
			if err != nil {
				return nil, err
			}
			args[i] = in
		}
	}

	return args, nil
}
