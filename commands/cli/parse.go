package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	fp "path"
	"runtime"
	"sort"
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

	// if -r is provided, and it is associated with the package builtin
	// recursive path option, allow recursive file paths
	recursiveOpt := req.Option(cmds.RecShort)
	recursive := false
	if recursiveOpt != nil && recursiveOpt.Definition() == cmds.OptionRecursivePath {
		recursive, _, err = recursiveOpt.Bool()
		if err != nil {
			return req, nil, nil, u.ErrCast()
		}
	}

	stringArgs, fileArgs, err := parseArgs(stringVals, stdin, cmd.Arguments, recursive)
	if err != nil {
		return req, cmd, path, err
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
	path := make([]string, 0, len(input))
	input2 := make([]string, 0, len(input))

	for i, blob := range input {
		if strings.HasPrefix(blob, "-") {
			input2 = append(input2, blob)
			continue
		}

		sub := cmd.Subcommand(blob)
		if sub == nil {
			input2 = append(input2, input[i:]...)
			break
		}
		cmd = sub
		path = append(path, blob)
	}

	return path, input2, cmd
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
	// ignore stdin on Windows
	if runtime.GOOS == "windows" {
		stdin = nil
	}

	// check if stdin is coming from terminal or is being piped in
	if stdin != nil {
		if term, err := isTerminal(stdin); err != nil {
			return nil, nil, err
		} else if term {
			stdin = nil // set to nil so we ignore it
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
	for i := 0; i < numInputs; i++ {
		argDef := getArgDef(argDefIndex, argDefs)

		// skip optional argument definitions if there aren't sufficient remaining inputs
		for numInputs-i <= numRequired && !argDef.Required {
			argDefIndex++
			argDef = getArgDef(argDefIndex, argDefs)
		}
		if argDef.Required {
			numRequired--
		}

		var err error
		if argDef.Type == cmds.ArgString {
			if stdin == nil {
				// add string values
				stringArgs, inputs = appendString(stringArgs, inputs)

			} else if argDef.SupportsStdin {
				// if we have a stdin, read it in and use the data as a string value
				stringArgs, stdin, err = appendStdinAsString(stringArgs, stdin)
				if err != nil {
					return nil, nil, err
				}
			}

		} else if argDef.Type == cmds.ArgFile {
			if stdin == nil {
				// treat stringArg values as file paths
				fileArgs, inputs, err = appendFile(fileArgs, inputs, argDef, recursive)
				if err != nil {
					return nil, nil, err
				}

			} else if argDef.SupportsStdin {
				// if we have a stdin, create a file from it
				fileArgs, stdin = appendStdinAsFile(fileArgs, stdin)
			}
		}

		argDefIndex++
	}

	// check to make sure we didn't miss any required arguments
	if len(argDefs) > argDefIndex {
		for _, argDef := range argDefs[argDefIndex:] {
			if argDef.Required {
				return nil, nil, fmt.Errorf("Argument '%s' is required", argDef.Name)
			}
		}
	}

	return stringArgs, fileArgs, nil
}

func getArgDef(i int, argDefs []cmds.Argument) *cmds.Argument {
	if i < len(argDefs) {
		// get the argument definition (usually just argDefs[i])
		return &argDefs[i]

	} else if len(argDefs) > 0 {
		// but if i > len(argDefs) we use the last argument definition)
		return &argDefs[len(argDefs)-1]
	}

	// only happens if there aren't any definitions
	return nil
}

func appendString(args, inputs []string) ([]string, []string) {
	return append(args, inputs[0]), inputs[1:]
}

func appendStdinAsString(args []string, stdin *os.File) ([]string, *os.File, error) {
	var buf bytes.Buffer

	_, err := buf.ReadFrom(stdin)
	if err != nil {
		return nil, nil, err
	}

	return append(args, buf.String()), nil, nil
}

func appendFile(args []cmds.File, inputs []string, argDef *cmds.Argument, recursive bool) ([]cmds.File, []string, error) {
	path := inputs[0]

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
				path, argDef.Name)
			return nil, nil, err
		}
		if !recursive {
			err = fmt.Errorf("'%s' is a directory, use the '-%s' flag to specify directories",
				path, cmds.RecShort)
			return nil, nil, err
		}
	}

	arg, err := openPath(file, path)
	if err != nil {
		return nil, nil, err
	}

	return append(args, arg), inputs[1:], nil
}

func appendStdinAsFile(args []cmds.File, stdin *os.File) ([]cmds.File, *os.File) {
	arg := &cmds.ReaderFile{"", stdin}
	return append(args, arg), nil
}

// recursively get file or directory contents as a cmds.File
func openPath(file *os.File, path string) (cmds.File, error) {
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

	// make sure contents are sorted so -- repeatably -- we get the same inputs.
	sort.Sort(sortFIByName(contents))

	files := make([]cmds.File, 0, len(contents))
	for _, child := range contents {
		childPath := fp.Join(path, child.Name())
		childFile, err := os.Open(childPath)
		if err != nil {
			return nil, err
		}

		f, err := openPath(childFile, childPath)
		if err != nil {
			return nil, err
		}

		files = append(files, f)
	}

	return &cmds.SliceFile{path, files}, nil
}

// isTerminal returns true if stdin is a Stdin pipe (e.g. `cat file | ipfs`),
// and false otherwise (e.g. nothing is being piped in, so stdin is
// coming from the terminal)
func isTerminal(stdin *os.File) (bool, error) {
	stat, err := stdin.Stat()
	if err != nil {
		return false, err
	}

	// if stdin is a CharDevice, return true
	return ((stat.Mode() & os.ModeCharDevice) != 0), nil
}

type sortFIByName []os.FileInfo

func (es sortFIByName) Len() int           { return len(es) }
func (es sortFIByName) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es sortFIByName) Less(i, j int) bool { return es[i].Name() < es[j].Name() }
