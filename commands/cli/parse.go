package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
	u "github.com/jbenet/go-ipfs/util"
)

// ErrInvalidSubcmd signals when the parse error is not found
var ErrInvalidSubcmd = errors.New("subcommand not found")

// Parse parses the input commandline string (cmd, flags, and args).
// returns the corresponding command Request object.
func Parse(input []string, stdin *os.File, root *cmds.Command) (cmds.Request, *cmds.Command, []string, error) {
	path, input, cmd := parsePath(input, root)
	if len(path) == 0 {
		return nil, nil, path, ErrInvalidSubcmd
	}

	opts, stringVals, err := parseOptions(input)
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

	req, err := cmds.NewRequest(path, opts, nil, nil, cmd, optDefs)
	if err != nil {
		return nil, cmd, path, err
	}

	recursive, _, err := req.Option(cmds.RecShort).Bool()
	if err != nil {
		return nil, nil, nil, u.ErrCast()
	}
	stringArgs, fileArgs, err := parseArgs(stringVals, stdin, cmd.Arguments, recursive)
	if err != nil {
		return nil, cmd, path, err
	}
	req.SetArguments(stringArgs)

	file := &cmds.SliceFile{"", fileArgs}
	req.SetFiles(file)

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

func parseArgs(inputs []string, stdin *os.File, argDefs []cmds.Argument, recursive bool) ([]string, []cmds.File, error) {
	// check if stdin is coming from terminal or is being piped in
	if stdin != nil {
		stat, err := stdin.Stat()
		if err != nil {
			return nil, nil, err
		}

		// if stdin isn't a CharDevice, set it to nil
		// (this means it is coming from terminal and we want to ignore it)
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			stdin = nil
		}
	}

	// count required argument definitions
	numRequired := 0
	for _, argDef := range argDefs {
		if argDef.Required {
			numRequired++
		}
	}

	// count number of values provided by user
	numInputs := len(inputs)
	if stdin != nil {
		numInputs += 1
	}

	// if we have more arg values provided than argument definitions,
	// and the last arg definition is not variadic (or there are no definitions), return an error
	notVariadic := len(argDefs) == 0 || !argDefs[len(argDefs)-1].Variadic
	if notVariadic && numInputs > len(argDefs) {
		return nil, nil, fmt.Errorf("Expected %v arguments, got %v", len(argDefs), numInputs)
	}

	stringArgs := make([]string, 0, numInputs)
	fileArgs := make([]cmds.File, 0, numInputs)

	argDefIndex := 0 // the index of the current argument definition
	for i, input := range inputs {
		// get the argument definiton (should be argDefs[argDefIndex],
		// but if argDefIndex > len(argDefs) we use the last argument definition)
		var argDef cmds.Argument
		if argDefIndex < len(argDefs) {
			argDef = argDefs[argDefIndex]
		} else if len(argDefs) > 0 {
			argDef = argDefs[len(argDefs)-1]
		}

		// skip optional argument definitions if there aren't sufficient remaining inputs
		if numInputs-i <= numRequired && !argDef.Required {
			continue
		} else if argDef.Required {
			numRequired--
		}

		if argDef.Type == cmds.ArgString {
			if stdin == nil {
				// add string values
				stringArgs = append(stringArgs, input)
				inputs = inputs[1:]

			} else if argDef.SupportsStdin {
				// if we have a stdin, read it in and use the data as a string value
				var buf bytes.Buffer
				_, err := buf.ReadFrom(stdin)
				if err != nil {
					return nil, nil, err
				}
				stringArgs = append(stringArgs, buf.String())
				stdin = nil
			}

		} else if argDef.Type == cmds.ArgFile {
			if stdin == nil {
				// treat stringArg values as file paths
				path := input
				inputs = inputs[1:]

				file, err := os.Open(path)
				if err != nil {
					return nil, nil, err
				}

				stat, err := file.Stat()
				if err != nil {
					return nil, nil, err
				}

				if stat.IsDir() {
					if !argDef.Recursive {
						err = fmt.Errorf("Invalid path '%s', argument '%s' does not support directories",
							input, argDef.Name)
						return nil, nil, err
					}
					if !recursive {
						err = fmt.Errorf("'%s' is a directory, use the '-%s' flag to specify directories",
							input, cmds.RecShort)
						return nil, nil, err
					}
				}

				fileArg, err := getFile(file, input)
				if err != nil {
					return nil, nil, err
				}

				fileArgs = append(fileArgs, fileArg)

			} else if argDef.SupportsStdin {
				// if we have a stdin, create a file from it
				fileArg := &cmds.ReaderFile{"", stdin}
				fileArgs = append(fileArgs, fileArg)
				stdin = nil
			}
		}

		argDefIndex++
	}

	return stringArgs, fileArgs, nil
}

// recursively get file or directory contents as a cmds.File
func getFile(file *os.File, path string) (cmds.File, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// for non-directories, return a ReaderFile
	if !stat.IsDir() {
		return &cmds.ReaderFile{path, file}, nil
	}

	// for directories, recursively iterate though children then return as a SliceFile
	contents, err := file.Readdir(0)
	if err != nil {
		return nil, err
	}

	files := make([]cmds.File, 0, len(contents))

	for _, child := range contents {
		childPath := fmt.Sprintf("%s/%s", path, child.Name())
		childFile, err := os.Open(childPath)
		if err != nil {
			return nil, err
		}

		f, err := getFile(childFile, childPath)
		if err != nil {
			return nil, err
		}

		files = append(files, f)
	}

	return &cmds.SliceFile{path, files}, nil
}
