package commands

import (
	"errors"
	"fmt"
	"strings"
  "io"

	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("command")

// Function is the type of function that Commands use.
// It reads from the Request, and writes results to the Response.
type Function func(Request, Response)

// Command is a runnable command, with input arguments and options (flags).
// It can also have subcommands, to group units of work into sets.
type Command struct {
	Help    string
	Options []Option

	run         Function
	subcommands map[string]*Command
}

// ErrNotCallable signals a command that cannot be called.
var ErrNotCallable = errors.New("This command can't be called directly. Try one of its subcommands.")

// Register adds a subcommand
func (c *Command) Register(id string, sub *Command) error {
	if c.subcommands == nil {
		c.subcommands = make(map[string]*Command)
	}

	// check for duplicate option names (only checks downwards)
	if err := checkOptionClashes(globalCommand, c, sub); err != nil {
		return err
	}

	if _, found := c.subcommands[id]; found {
		return fmt.Errorf("There is already a subcommand registered with id '%s'", id)
	}

	c.subcommands[id] = sub
	return nil
}

// Call invokes the command for the given Request
// Streaming output is written to `out`
func (c *Command) Call(req Request, out io.Writer) Response {
	res := NewResponse(req, out)

	cmds, err := c.Resolve(req.Path())
	if err != nil {
		res.SetError(err, ErrClient)
		return res
	}
	cmd := cmds[len(cmds)-1]

	if cmd.run == nil {
		res.SetError(ErrNotCallable, ErrClient)
		return res
	}

	options, err := c.GetOptions(req.Path())
	if err != nil {
		res.SetError(err, ErrClient)
		return res
	}

	err = req.ConvertOptions(options)
	if err != nil {
		res.SetError(err, ErrClient)
		return res
	}

	cmd.run(req, res)

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
	copy(options, c.Options)
	options = append(options, globalOptions...)

	cmds, err := c.Resolve(path)
	if err != nil {
		return nil, err
	}
	for _, cmd := range cmds {
		options = append(options, cmd.Options...)
	}

	optionsMap := make(map[string]Option)
	for _, opt := range options {
		for _, name := range opt.Names {
			optionsMap[name] = opt
		}
	}

	return optionsMap, nil
}

// Subcommand returns the subcommand with the given id
func (c *Command) Subcommand(id string) *Command {
	return c.subcommands[id]
}

// AddOptionNames returns a map of all command options names, and the command
// they belong to. Will error if names clash in the command hierarchy.
func AddOptionNames(c *Command, names map[string]*Command) error {

	for _, opt := range c.Options {
		for _, name := range opt.Names {
			if c2, found := names[name]; found {

				// option can be reused in same command, but more often than not
				// the clash will be across commands so error out with that, as
				// commands tell us where the problem is
				errstr := "Option name ('%s') used multiple times (%v, %v)"
				return fmt.Errorf(errstr, c2, c)
			}

			// mark the name as in use
			names[name] = c
		}
	}

	// for every subcommand, recurse
	for _, c2 := range c.subcommands {
		if err := AddOptionNames(c2, names); err != nil {
			return err
		}
	}

	return nil
}

// checkOptionClashes checks all command option names for clashes
func checkOptionClashes(cmds ...*Command) error {
	names := map[string]*Command{}

	for _, c := range cmds {
		if err := AddOptionNames(c, names); err != nil {
			return err
		}
	}

	return nil
}
