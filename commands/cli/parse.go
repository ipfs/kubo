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
	var root, cmd *cmds.Command
	var path, stringArgs []string
	var opts map[string]interface{}

	// use the root that matches the longest path (most accurately matches request)
	maxLength := 0
	for _, r := range roots {
		p, i, c := parsePath(input, r)
		o, s, err := parseOptions(i)
		if err != nil {
			return nil, nil, err
		}

		length := len(p)
		if length > maxLength {
			maxLength = length
			root = r
			path = p
			cmd = c
			opts = o
			stringArgs = s
		}
	}

	if maxLength == 0 {
		return nil, nil, errors.New("Not a valid subcommand")
	}

	args, err := parseArgs(stringArgs, cmd)
	if err != nil {
		return nil, nil, err
	}

	req := cmds.NewRequest(path, opts, args, cmd)

	err = cmd.CheckArguments(req)
	if err != nil {
		return nil, nil, err
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

func parseArgs(stringArgs []string, cmd *cmds.Command) ([]interface{}, error) {
	var argDef cmds.Argument
	args := make([]interface{}, len(stringArgs))

	for i, arg := range stringArgs {
		if i < len(cmd.Arguments) {
			argDef = cmd.Arguments[i]
		}

		if argDef.Type == cmds.ArgString {
			args[i] = arg

		} else {
			in, err := os.Open(arg)
			if err != nil {
				return nil, err
			}
			args[i] = in
		}
	}

	return args, nil
}
