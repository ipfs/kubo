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
func Parse(input []string, roots ...*cmds.Command) (cmds.Request, *cmds.Command, *cmds.Command, error) {
	var root, cmd *cmds.Command
	var path, stringArgs []string
	var opts map[string]interface{}

	// use the root that matches the longest path (most accurately matches request)
	maxLength := 0
	for _, r := range roots {
		p, i, c := parsePath(input, r)
		o, s, err := parseOptions(i)
		if err != nil {
			return nil, root, c, err
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
		return nil, root, nil, errors.New("Not a valid subcommand")
	}

	args, err := parseArgs(stringArgs, cmd)
	if err != nil {
		return nil, root, cmd, err
	}

	req := cmds.NewRequest(path, opts, args, cmd)

	err = cmd.CheckArguments(req)
	if err != nil {
		return nil, root, cmd, err
	}

	return req, root, cmd, nil
}

// parsePath separates the command path and the opts and args from a command string
// returns command path slice, rest slice, and the corresponding *cmd.Command
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
	args := make([]interface{}, 0)

	// count required argument definitions
	lenRequired := 0
	for _, argDef := range cmd.Arguments {
		if argDef.Required {
			lenRequired++
		}
	}

	j := 0
	for _, argDef := range cmd.Arguments {
		// skip optional argument definitions if there aren't sufficient remaining values
		if len(stringArgs)-j <= lenRequired && !argDef.Required {
			continue
		} else if argDef.Required {
			lenRequired--
		}

		if j >= len(stringArgs) {
			break
		}

		if argDef.Variadic {
			for _, arg := range stringArgs[j:] {
				var err error
				args, err = appendArg(args, argDef, arg)
				if err != nil {
					return nil, err
				}
				j++
			}
		} else {
			var err error
			args, err = appendArg(args, argDef, stringArgs[j])
			if err != nil {
				return nil, err
			}
			j++
		}
	}

	if len(stringArgs)-j > 0 {
		args = append(args, make([]interface{}, len(stringArgs)-j))
	}

	return args, nil
}

func appendArg(args []interface{}, argDef cmds.Argument, value string) ([]interface{}, error) {
	if argDef.Type == cmds.ArgString {
		return append(args, value), nil

	} else {
		in, err := os.Open(value)
		if err != nil {
			return nil, err
		}
		return append(args, in), nil
	}
}
