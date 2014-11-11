package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("command")

// Function is the type of function that Commands use.
// It reads from the Request, and writes results to the Response.
type Function func(Request) (interface{}, error)

// Marshaller is a function that takes in a Response, and returns a marshalled []byte
// (or an error on failure)
type Marshaller func(Response) ([]byte, error)

// TODO: check Argument definitions when creating a Command
//   (might need to use a Command constructor)
//   * make sure any variadic args are at the end
//   * make sure there aren't duplicate names
//   * make sure optional arguments aren't followed by required arguments

// Command is a runnable command, with input arguments and options (flags).
// It can also have Subcommands, to group units of work into sets.
type Command struct {
	Description string
	Help        string

	Options     []Option
	Arguments   []Argument
	Run         Function
	Marshallers map[EncodingType]Marshaller
	Type        interface{}
	Subcommands map[string]*Command
}

// ErrNotCallable signals a command that cannot be called.
var ErrNotCallable = errors.New("This command can't be called directly. Try one of its subcommands.")

var ErrNoFormatter = errors.New("This command cannot be formatted to plain text")

// Call invokes the command for the given Request
func (c *Command) Call(req Request) Response {
	res := NewResponse(req)

	cmds, err := c.Resolve(req.Path())
	if err != nil {
		res.SetError(err, ErrClient)
		return res
	}
	cmd := cmds[len(cmds)-1]

	if cmd.Run == nil {
		res.SetError(ErrNotCallable, ErrClient)
		return res
	}

	err = cmd.CheckArguments(req)
	if err != nil {
		res.SetError(err, ErrClient)
		return res
	}

	err = req.ConvertOptions()
	if err != nil {
		res.SetError(err, ErrClient)
		return res
	}

	output, err := cmd.Run(req)
	if err != nil {
		// if returned error is a commands.Error, use its error code
		// otherwise, just default the code to ErrNormal
		var e Error
		e, ok := err.(Error)
		if ok {
			res.SetError(e, e.Code)
		} else {
			res.SetError(err, ErrNormal)
		}
		return res
	}

	res.SetOutput(output)
	return res
}

// Resolve gets the subcommands at the given path
func (c *Command) Resolve(path []string) ([]*Command, error) {
	cmds := make([]*Command, len(path)+1)
	cmds[0] = c

	cmd := c
	for i, name := range path {
		cmd = cmd.Subcommand(name)

		if cmd == nil {
			pathS := strings.Join(path[0:i], "/")
			return nil, fmt.Errorf("Undefined command: '%s'", pathS)
		}

		cmds[i+1] = cmd
	}

	return cmds, nil
}

// Get resolves and returns the Command addressed by path
func (c *Command) Get(path []string) (*Command, error) {
	cmds, err := c.Resolve(path)
	if err != nil {
		return nil, err
	}
	return cmds[len(cmds)-1], nil
}

// GetOptions gets the options in the given path of commands
func (c *Command) GetOptions(path []string) (map[string]Option, error) {
	options := make([]Option, len(c.Options))

	cmds, err := c.Resolve(path)
	if err != nil {
		return nil, err
	}
	cmds = append(cmds, globalCommand)

	for _, cmd := range cmds {
		options = append(options, cmd.Options...)
	}

	optionsMap := make(map[string]Option)
	for _, opt := range options {
		for _, name := range opt.Names {
			if _, found := optionsMap[name]; found {
				return nil, fmt.Errorf("Option name '%s' used multiple times", name)
			}

			optionsMap[name] = opt
		}
	}

	return optionsMap, nil
}

func (c *Command) CheckArguments(req Request) error {
	args := req.Arguments()
	argDefs := c.Arguments

	// if we have more arg values provided than argument definitions,
	// and the last arg definition is not variadic (or there are no definitions), return an error
	notVariadic := len(argDefs) == 0 || !argDefs[len(argDefs)-1].Variadic
	if notVariadic && len(args) > len(argDefs) {
		return fmt.Errorf("Expected %v arguments, got %v", len(argDefs), len(args))
	}

	// count required argument definitions
	lenRequired := 0
	for _, argDef := range c.Arguments {
		if argDef.Required {
			lenRequired++
		}
	}

	// iterate over the arg definitions
	j := 0
	for _, argDef := range c.Arguments {
		// skip optional argument definitions if there aren't sufficient remaining values
		if len(args)-j <= lenRequired && !argDef.Required {
			continue
		}

		// the value for this argument definition. can be nil if it wasn't provided by the caller
		var v interface{}
		if j < len(args) {
			v = args[j]
			j++
		}

		err := checkArgValue(v, argDef)
		if err != nil {
			return err
		}

		// any additional values are for the variadic arg definition
		if argDef.Variadic && j < len(args)-1 {
			for _, val := range args[j:] {
				err := checkArgValue(val, argDef)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Subcommand returns the subcommand with the given id
func (c *Command) Subcommand(id string) *Command {
	return c.Subcommands[id]
}

// checkArgValue returns an error if a given arg value is not valid for the given Argument
func checkArgValue(v interface{}, def Argument) error {
	if v == nil {
		if def.Required {
			return fmt.Errorf("Argument '%s' is required", def.Name)
		}

		return nil
	}

	if def.Type == ArgFile {
		_, ok := v.(io.Reader)
		if !ok {
			return fmt.Errorf("Argument '%s' isn't valid", def.Name)
		}

	} else if def.Type == ArgString {
		_, ok := v.(string)
		if !ok {
			return fmt.Errorf("Argument '%s' must be a string", def.Name)
		}
	}

	return nil
}
