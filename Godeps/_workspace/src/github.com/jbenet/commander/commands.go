// Copyright 2012 The Go-Commander Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Based on the original work by The Go Authors:
// Copyright 2011 The Go Authors.  All rights reserved.

// commander helps creating command line programs whose arguments are flags,
// commands and subcommands.
package commander

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
)

// UsageSection differentiates between sections in the usage text.
type Listing int

const (
	CommandsList = iota
	HelpTopicsList
	Unlisted
)

// A Command is an implementation of a subcommand.
type Command struct {

	// UsageLine is the short usage message.
	// The first word in the line is taken to be the command name.
	UsageLine string

	// Short is the short description line shown in command lists.
	Short string

	// Long is the long description shown in the 'help <this-command>' output.
	Long string

	// List reports which list to show this command in Usage and Help.
	// Choose between {CommandsList (default), HelpTopicsList, Unlisted}
	List Listing

	// Run runs the command.
	// The args are the arguments after the command name.
	Run func(cmd *Command, args []string) error

	// Flag is a set of flags specific to this command.
	Flag flag.FlagSet

	// CustomFlags indicates that the command will do its own
	// flag parsing.
	CustomFlags bool

	// Subcommands are dispatched from this command
	Subcommands []*Command

	// Parent command, nil for root.
	Parent *Command

	// UsageTemplate formats the usage (short) information displayed to the user
	// (leave empty for default)
	UsageTemplate string

	// HelpTemplate formats the help (long) information displayed to the user
	// (leave empty for default)
	HelpTemplate string

	// Stdout and Stderr by default are os.Stdout and os.Stderr, but you can
	// point them at any io.Writer
	Stdout io.Writer
	Stderr io.Writer
}

// Name returns the command's name: the first word in the usage line.
func (c *Command) Name() string {
	name := c.UsageLine
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

// Usage prints the usage details to the standard error output.
func (c *Command) Usage() {
	c.usage()
}

// FlagOptions returns the flag's options as a string
func (c *Command) FlagOptions() string {
	var buf bytes.Buffer
	c.Flag.SetOutput(&buf)
	c.Flag.PrintDefaults()

	str := string(buf.Bytes())
	if len(str) > 0 {
		return fmt.Sprintf("\nOptions:\n%s", str)
	}
	return ""
}

// Runnable reports whether the command can be run; otherwise
// it is a documentation pseudo-command such as importpath.
func (c *Command) Runnable() bool {
	return c.Run != nil
}

// Type to allow us to use sort.Sort on a slice of Commands
type CommandSlice []*Command

func (c CommandSlice) Len() int {
	return len(c)
}

func (c CommandSlice) Less(i, j int) bool {
	return c[i].Name() < c[j].Name()
}

func (c CommandSlice) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// Sort the commands
func (c *Command) SortCommands() {
	sort.Sort(CommandSlice(c.Subcommands))
}

// Init the command
func (c *Command) init() {
	if c.Parent != nil {
		return // already initialized.
	}

	// setup strings
	if len(c.UsageLine) < 1 {
		c.UsageLine = Defaults.UsageLine
	}
	if len(c.UsageTemplate) < 1 {
		c.UsageTemplate = Defaults.UsageTemplate
	}
	if len(c.HelpTemplate) < 1 {
		c.HelpTemplate = Defaults.HelpTemplate
	}

	if c.Stderr == nil {
		c.Stderr = os.Stderr
	}
	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}

	// init subcommands
	for _, cmd := range c.Subcommands {
		cmd.init()
	}

	// init hierarchy...
	for _, cmd := range c.Subcommands {
		cmd.Parent = c
	}
}

// Dispatch executes the command using the provided arguments.
// If a subcommand exists matching the first argument, it is dispatched.
// Otherwise, the command's Run function is called.
func (c *Command) Dispatch(args []string) error {
	if c == nil {
		return fmt.Errorf("Called Run() on a nil Command")
	}

	// Ensure command is initialized.
	c.init()

	// First, try a sub-command
	if len(args) > 0 {
		for _, cmd := range c.Subcommands {
			n := cmd.Name()
			if n == args[0] {
				return cmd.Dispatch(args[1:])
			}
		}

		// help is builtin (but after, to allow overriding)
		if args[0] == "help" {
			return c.help(args[1:])
		}

		// then, try out an external binary (git-style)
		bin, err := exec.LookPath(c.FullName() + "-" + args[0])
		if err == nil {
			cmd := exec.Command(bin, args[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = c.Stdout
			cmd.Stderr = c.Stderr
			return cmd.Run()
		}
	}

	// then, try running this command
	if c.Runnable() {
		if !c.CustomFlags {
			var err = error(nil)
			c.Flag.Usage = func() {
				c.Usage()
				err = fmt.Errorf("Failed to parse flags.")
			}
			c.Flag.Parse(args)
			if err != nil {
				return err
			}
			args = c.Flag.Args()
		}
		return c.Run(c, args)
	}

	// TODO: try an alias
	//...

	// Last, print usage
	if err := c.usage(); err != nil {
		return err
	}
	return nil
}

func (c *Command) usage() error {
	// c.SortCommands()
	err := tmpl(c.Stderr, c.UsageTemplate, c)
	if err != nil {
		fmt.Println(err)
	}
	return err
}

// help implements the 'help' command.
func (c *Command) help(args []string) error {

	// help exactly for this command?
	if len(args) == 0 {
		if len(c.Long) > 0 {
			return tmpl(c.Stdout, c.HelpTemplate, c)
		} else {
			return c.usage()
		}
	}

	arg := args[0]

	// is this help for a subcommand?
	for _, cmd := range c.Subcommands {
		n := cmd.Name()
		// strip out "<parent>-"" name
		if strings.HasPrefix(n, c.Name()+"-") {
			n = n[len(c.Name()+"-"):]
		}
		if n == arg {
			return cmd.help(args[1:])
		}
	}

	return fmt.Errorf("Unknown help topic %#q.  Run '%v help'.\n", arg, c.Name())
}

func (c *Command) MaxLen() (res int) {
	res = 0
	for _, cmd := range c.Subcommands {
		i := len(cmd.Name())
		if i > res {
			res = i
		}
	}
	return
}

// ColFormat returns the column header size format for printing in the template
func (c *Command) ColFormat() string {
	sz := c.MaxLen()
	if sz < 11 {
		sz = 11
	}
	return fmt.Sprintf("%%-%ds", sz)
}

// FullName returns the full name of the command, prefixed with parent commands
func (c *Command) FullName() string {
	n := c.Name()
	if c.Parent != nil {
		n = c.Parent.FullName() + "-" + n
	}
	return n
}

// FullSpacedName returns the full name of the command, with ' ' instead of '-'
func (c *Command) FullSpacedName() string {
	n := c.Name()
	if c.Parent != nil {
		n = c.Parent.FullSpacedName() + " " + n
	}
	return n
}

func (c *Command) SubcommandList(list Listing) []*Command {
	var cmds []*Command
	for _, cmd := range c.Subcommands {
		if cmd.List == list {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

var Defaults = Command{
	UsageTemplate: `{{if .Runnable}}Usage: {{if .Parent}}{{.Parent.FullSpacedName}}{{end}} {{.UsageLine}}

{{else}}{{.FullSpacedName}} - {{end}}{{.Short}}

{{if commandList}}Commands:
{{range commandList}}
    {{.Name | printf (colfmt)}} {{.Short}}{{end}}

Use "{{.Name}} help <command>" for more information about a command.

{{end}}{{.FlagOptions}}{{if helpList}}
Additional help topics:
{{range helpList}}
    {{.Name | printf (colfmt)}} {{.Short}}{{end}}

Use "{{.Name}} help <topic>" for more information about that topic.

{{end}}`,

	HelpTemplate: `{{if .Runnable}}Usage: {{if .Parent}}{{.Parent.FullSpacedName}}{{end}} {{.UsageLine}}

{{end}}{{.Long | trim}}
{{.FlagOptions}}
`,
}

// tmpl executes the given template text on data, writing the result to w.
func tmpl(w io.Writer, text string, data interface{}) error {
	t := template.New("top")
	t.Funcs(template.FuncMap{
		"trim":        strings.TrimSpace,
		"colfmt":      func() string { return data.(*Command).ColFormat() },
		"commandList": func() []*Command { return data.(*Command).SubcommandList(CommandsList) },
		"helpList":    func() []*Command { return data.(*Command).SubcommandList(HelpTopicsList) },
	})
	template.Must(t.Parse(text))
	return t.Execute(w, data)
}
