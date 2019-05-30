// Package commands implements the ipfs command interface
//
// Using github.com/ipfs/go-ipfs/commands to define the command line and HTTP
// APIs.  This is the interface available to folks using IPFS from outside of
// the Go language.
package commands

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ipfs/go-ipfs-cmds"
)

type commandEncoder struct {
	w io.Writer
}

func (e *commandEncoder) Encode(v interface{}) error {
	var (
		cmd *Command
		ok  bool
	)

	if cmd, ok = v.(*Command); !ok {
		return fmt.Errorf(`core/commands: uenxpected type %T, expected *"core/commands".Command`, v)
	}

	for _, s := range cmdPathStrings(cmd, cmd.showOpts) {
		_, err := e.w.Write([]byte(s + "\n"))
		if err != nil {
			return err
		}
	}

	return nil
}

type Command struct {
	Name        string
	Subcommands []Command
	Options     []Option

	showOpts bool
}

type Option struct {
	Names []string
}

const (
	flagsOptionName = "flags"
)

// CommandsCmd takes in a root command,
// and returns a command that lists the subcommands in that root
func CommandsCmd(root *cmds.Command) *cmds.Command {
	return &cmds.Command{
		Helptext: cmds.HelpText{
			Tagline:          "List all available commands.",
			ShortDescription: `Lists all available commands (and subcommands) and exits.`,
		},
		Options: []cmds.Option{
			cmds.BoolOption(flagsOptionName, "f", "Show command flags"),
		},
		Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
			rootCmd := cmd2outputCmd("ipfs", root)
			rootCmd.showOpts, _ = req.Options[flagsOptionName].(bool)
			return cmds.EmitOnce(res, &rootCmd)
		},
		Encoders: cmds.EncoderMap{
			cmds.Text: func(req *cmds.Request) func(io.Writer) cmds.Encoder {
				return func(w io.Writer) cmds.Encoder { return &commandEncoder{w} }
			},
		},
		Type: Command{},
	}
}

func cmd2outputCmd(name string, cmd *cmds.Command) Command {
	opts := make([]Option, len(cmd.Options))
	for i, opt := range cmd.Options {
		opts[i] = Option{opt.Names()}
	}

	output := Command{
		Name:        name,
		Subcommands: make([]Command, 0, len(cmd.Subcommands)),
		Options:     opts,
	}

	for name, sub := range cmd.Subcommands {
		output.Subcommands = append(output.Subcommands, cmd2outputCmd(name, sub))
	}

	return output
}

func cmdPathStrings(cmd *Command, showOptions bool) []string {
	var cmds []string

	var recurse func(prefix string, cmd *Command)
	recurse = func(prefix string, cmd *Command) {
		newPrefix := prefix + cmd.Name
		cmds = append(cmds, newPrefix)
		if prefix != "" && showOptions {
			for _, options := range cmd.Options {
				var cmdOpts []string
				for _, flag := range options.Names {
					if len(flag) == 1 {
						flag = "-" + flag
					} else {
						flag = "--" + flag
					}
					cmdOpts = append(cmdOpts, newPrefix+" "+flag)
				}
				cmds = append(cmds, strings.Join(cmdOpts, " / "))
			}
		}
		for _, sub := range cmd.Subcommands {
			recurse(newPrefix+" ", &sub)
		}
	}

	recurse("", cmd)
	sort.Strings(cmds)
	return cmds
}

type nonFatalError string

// streamResult is a helper function to stream results that possibly
// contain non-fatal errors.  The helper function is allowed to panic
// on internal errors.
func streamResult(procVal func(interface{}, io.Writer) nonFatalError) func(cmds.Response, cmds.ResponseEmitter) error {
	return func(res cmds.Response, re cmds.ResponseEmitter) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("internal error: %v", r)
			}
			re.Close()
		}()

		var errors bool
		for {
			v, err := res.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			errorMsg := procVal(v, os.Stdout)

			if errorMsg != "" {
				errors = true
				fmt.Fprintf(os.Stderr, "%s\n", errorMsg)
			}
		}

		if errors {
			return fmt.Errorf("errors while displaying some entries")
		}
		return nil
	}
}
