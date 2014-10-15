package commands

import (
	"fmt"
	"strings"
)

type Command struct {
	Help        string
	Options     []Option
	f           func(*Request, *Response)
	subcommands map[string]*Command
}

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
	cmd := c
	res := &Response{req: req}

  options, err := cmd.GetOptions(req.path)
  if err != nil {
    res.SetError(err, Client)
    return res
  }

	err = req.convertOptions(options)
  if err != nil {
    res.SetError(err, Client)
    return res
  }

	cmd.f(req, res)

	return res
}

// GetOptions gets the options in the given path of commands
func (c *Command) GetOptions(path []string) (map[string]Option, error) {
  options := make([]Option, len(c.Options))
  copy(options, c.Options)
  options = append(options, globalOptions...)

  // a nil path means this command, not a subcommand (same as an empty path)
  if path != nil {
    for i, id := range path {
      cmd := c.Sub(id)

      if cmd == nil {
        pathS := strings.Join(path[0:i], "/")
        return nil, fmt.Errorf("Undefined command: '%s'", pathS)
      }

      options = append(options, cmd.Options...)
    }
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
