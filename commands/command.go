package commands

import (
  "fmt"
  "strings"
  "reflect"
)

type Command struct {
  Help string
  Options []Option
  f func(*Request) (interface{}, error)
  subcommands map[string]*Command
}

// Register adds a subcommand
func (c *Command) Register(id string, sub *Command) error {
  if c.subcommands == nil {
    c.subcommands = make(map[string]*Command)
  }

  // check for duplicate option names (only checks downwards)
  names := make(map[string]bool)
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
func (c *Command) Call(path []string, req *Request) (interface{}, error) {
  options := make([]Option, len(c.Options))
  copy(options, c.Options)
  cmd := c

  if path != nil {
    for i, id := range path {
      cmd = c.Sub(id)

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

  for k, v := range req.options {
    opt, ok := optionsMap[k]

    if !ok {
      return nil, fmt.Errorf("Unrecognized command option: '%s'", k)
    }

    for _, name := range opt.Names {
      if _, ok = req.options[name]; name != k && ok {
        return nil, fmt.Errorf("Duplicate command options were provided ('%s' and '%s')",
          k, name)
      }
    }

    kind := reflect.TypeOf(v).Kind()
    if kind != opt.Type {
      return nil, fmt.Errorf("Option '%s' should be type '%s', but got type '%s'",
        k, opt.Type.String(), kind.String())
    }
  }

  return cmd.f(req)
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
