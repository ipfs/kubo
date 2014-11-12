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
// Multiple root commands are supported:
// Parse will search each root to find the one that best matches the requested subcommand.
func Parse(input []string, roots ...*cmds.Command) (cmds.Request, *cmds.Command, *cmds.Command, []string, error) {
	var root, cmd *cmds.Command
	var path, stringArgs []string
	var opts map[string]interface{}

	// use the root that matches the longest path (most accurately matches request)
	maxLength := 0
	for _, root2 := range roots {
		path2, input2, cmd2 := parsePath(input, root2)
		opts2, stringArgs2, err := parseOptions(input2)
		if err != nil {
			return nil, root, cmd2, path2, err
		}

		length := len(path2)
		if length > maxLength {
			maxLength = length
			root = root2
			path = path2
			cmd = cmd2
			opts = opts2
			stringArgs = stringArgs2
		}
	}

	if maxLength == 0 {
		return nil, root, nil, path, errors.New("Not a valid subcommand")
	}

	args, err := parseArgs(stringArgs, cmd)
	if err != nil {
		return nil, root, cmd, path, err
	}

	optDefs, err := root.GetOptions(path)
	if err != nil {
		return nil, root, cmd, path, err
	}

	req := cmds.NewRequest(path, opts, args, cmd, optDefs)

	err = cmd.CheckArguments(req)
	if err != nil {
		return req, root, cmd, path, err
	}

	return req, root, cmd, path, nil
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
