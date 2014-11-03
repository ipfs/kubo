package cli

import (
	"errors"
	"fmt"
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
		opts, args, err := parseOptions(input)
		if err != nil {
			return nil, nil, err
		}

		length := len(path)
		if length > maxLength {
			maxLength = length
			req = cmds.NewRequest(path, opts, args, cmd)
			root = r
		}
	}

	if maxLength == 0 {
		return nil, nil, errors.New("Not a valid subcommand")
	}

	// TODO: figure out how to know when to read given file(s) as an input stream
	// (instead of filename arg string)

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
func parseOptions(input []string) (map[string]interface{}, []interface{}, error) {
	opts := make(map[string]interface{})
	args := []interface{}{}

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
