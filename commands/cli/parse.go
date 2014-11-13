package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
)

// ErrInvalidSubcmd signals when the parse error is not found
var ErrInvalidSubcmd = errors.New("subcommand not found")

// Parse parses the input commandline string (cmd, flags, and args).
// returns the corresponding command Request object.
// Parse will search each root to find the one that best matches the requested subcommand.
func Parse(input []string, root *cmds.Command) (cmds.Request, *cmds.Command, []string, error) {
	// use the root that matches the longest path (most accurately matches request)
	path, input, cmd := parsePath(input, root)
	opts, stringArgs, err := parseOptions(input)
	if err != nil {
		return nil, cmd, path, err
	}

	if len(path) == 0 {
		return nil, nil, path, ErrInvalidSubcmd
	}

	args, err := parseArgs(stringArgs, cmd.Arguments)
	if err != nil {
		return nil, cmd, path, err
	}

	optDefs, err := root.GetOptions(path)
	if err != nil {
		return nil, cmd, path, err
	}

	req := cmds.NewRequest(path, opts, args, cmd, optDefs)

	err = cmd.CheckArguments(req)
	if err != nil {
		return req, cmd, path, err
	}

	return req, cmd, path, nil
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

func parseArgs(stringArgs []string, arguments []cmds.Argument) ([]interface{}, error) {
	// count required argument definitions
	lenRequired := 0
	for _, argDef := range arguments {
		if argDef.Required {
			lenRequired++
		}
	}

	args := make([]interface{}, len(stringArgs))

	valueIndex := 0 // the index of the current stringArgs value
	for _, argDef := range arguments {
		// skip optional argument definitions if there aren't sufficient remaining values
		if len(stringArgs)-valueIndex <= lenRequired && !argDef.Required {
			continue
		} else if argDef.Required {
			lenRequired--
		}

		if valueIndex >= len(stringArgs) {
			break
		}

		if argDef.Variadic {
			for _, arg := range stringArgs[valueIndex:] {
				value, err := argValue(argDef, arg)
				if err != nil {
					return nil, err
				}
				args[valueIndex] = value
				valueIndex++
			}
		} else {
			var err error
			value, err := argValue(argDef, stringArgs[valueIndex])
			if err != nil {
				return nil, err
			}
			args[valueIndex] = value
			valueIndex++
		}
	}

	return args, nil
}

func argValue(argDef cmds.Argument, value string) (interface{}, error) {
	if argDef.Type == cmds.ArgString {
		return value, nil

	} else {
		// NB At the time of this commit, file cleanup is performed when
		// Requests are cleaned up. TODO try to perform open and close at the
		// same level of abstraction (or at least in the same package!)
		in, err := os.Open(value)
		if err != nil {
			return nil, err
		}
		return in, nil
	}
}
