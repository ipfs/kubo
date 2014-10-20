package commands

import (
	"errors"
	"fmt"
	"strings"
)

// Command is an object that defines a command.
type Command struct {
	Help        string
	Options     []Option
	f           func(*Request, *Response)
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
	names := make(map[string]bool)
	globalCommand.checkOptions(names)
	c.checkOptions(names)
	err := sub.checkOptions(names)
	if err != nil {
		return err
	}

	if _, ok := c.subcommands[id]; ok {
		return fmt.Errorf("There is already a subcommand registered with id '%s'", id)
	}

	c.subcommands[id] = sub
	return nil
}

// Call invokes the command at the given subcommand path
func (c *Command) Call(req *Request) *Response {
	res := &Response{req: req}

	cmds, err := c.Resolve(req.path)
	if err != nil {
		res.SetError(err, ErrClient)
		return res
	}
	cmd := cmds[len(cmds)-1]

	if cmd.f == nil {
		res.SetError(ErrNotCallable, ErrClient)
		return res
	}

	options, err := c.GetOptions(req.path)
	if err != nil {
		res.SetError(err, ErrClient)
		return res
	}

	err = req.convertOptions(options)
	if err != nil {
		res.SetError(err, ErrClient)
		return res
	}

	cmd.f(req, res)

	return res
}

// Resolve gets the subcommands at the given path
func (c *Command) Resolve(path []string) ([]*Command, error) {
	cmds := make([]*Command, len(path)+1)
	cmds[0] = c

	cmd := c
	for i, name := range path {
		cmd = cmd.Sub(name)

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

// Sub returns the subcommand with the given id
func (c *Command) Sub(id string) *Command {
	return c.subcommands[id]
}

func (c *Command) checkOptions(names map[string]bool) error {
	for _, opt := range c.Options {
		for _, name := range opt.Names {
			if _, ok := names[name]; ok {
				return fmt.Errorf("Multiple options are using the same name ('%s')", name)
			}
			names[name] = true
		}
	}

	for _, cmd := range c.subcommands {
		err := cmd.checkOptions(names)
		if err != nil {
			return err
		}
	}

	return nil
}
