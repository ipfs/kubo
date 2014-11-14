package cli

import (
	"bytes"
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
func Parse(input []string, stdin *os.File, root *cmds.Command) (cmds.Request, *cmds.Command, []string, error) {
	// use the root that matches the longest path (most accurately matches request)
	path, input, cmd := parsePath(input, root)
	opts, stringArgs, err := parseOptions(input)
	if err != nil {
		return nil, cmd, path, err
	}

	if len(path) == 0 {
		return nil, nil, path, ErrInvalidSubcmd
	}

	args, err := parseArgs(stringArgs, stdin, cmd.Arguments)
	if err != nil {
		return nil, cmd, path, err
	}

	optDefs, err := root.GetOptions(path)
	if err != nil {
		return nil, cmd, path, err
	}

	// check to make sure there aren't any undefined options
	for k := range opts {
		if _, found := optDefs[k]; !found {
			err = fmt.Errorf("Unrecognized option: -%s", k)
			return nil, cmd, path, err
		}
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

func parseArgs(stringArgs []string, stdin *os.File, arguments []cmds.Argument) ([]interface{}, error) {
	// check if stdin is coming from terminal or is being piped in
	if stdin != nil {
		stat, err := stdin.Stat()
		if err != nil {
			return nil, err
		}

		// if stdin isn't a CharDevice, set it to nil
		// (this means it is coming from terminal and we want to ignore it)
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			stdin = nil
		}
	}

	// count required argument definitions
	lenRequired := 0
	for _, argDef := range arguments {
		if argDef.Required {
			lenRequired++
		}
	}

	valCount := len(stringArgs)
	if stdin != nil {
		valCount += 1
	}

	args := make([]interface{}, 0, valCount)

	argDefIndex := 0 // the index of the current argument definition
	for i := 0; i < valCount; i++ {
		// get the argument definiton (should be arguments[argDefIndex],
		// but if argDefIndex > len(arguments) we use the last argument definition)
		var argDef cmds.Argument
		if argDefIndex < len(arguments) {
			argDef = arguments[argDefIndex]
		} else if len(arguments) > 0 {
			argDef = arguments[len(arguments)-1]
		}

		// skip optional argument definitions if there aren't sufficient remaining values
		if valCount-i <= lenRequired && !argDef.Required {
			continue
		} else if argDef.Required {
			lenRequired--
		}

		if argDef.Type == cmds.ArgString {
			if stdin == nil {
				// add string values
				args = append(args, stringArgs[0])
				stringArgs = stringArgs[1:]

			} else if argDef.SupportsStdin {
				// if we have a stdin, read it in and use the data as a string value
				var buf bytes.Buffer
				_, err := buf.ReadFrom(stdin)
				if err != nil {
					return nil, err
				}
				args = append(args, buf.String())
				stdin = nil
			}

		} else if argDef.Type == cmds.ArgFile {
			if stdin == nil {
				// treat stringArg values as file paths
				file, err := os.Open(stringArgs[0])
				if err != nil {
					return nil, err
				}
				args = append(args, file)
				stringArgs = stringArgs[1:]

			} else if argDef.SupportsStdin {
				// if we have a stdin, use that as a reader
				args = append(args, stdin)
				stdin = nil
			}
		}

		argDefIndex++
	}

	return args, nil
}
