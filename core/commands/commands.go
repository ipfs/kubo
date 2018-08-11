// Package commands implements the ipfs command interface
//
// Using github.com/ipfs/go-ipfs/commands to define the command line and HTTP
// APIs.  This is the interface available to folks using IPFS from outside of
// the Go language.
package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	e "github.com/ipfs/go-ipfs/core/commands/e"

	"gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmUQb3xtNzkQCgTj2NjaqcJZNv2nfSSub2QAdy9DtQMRBT/go-ipfs-cmds"
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
		Helptext: cmdkit.HelpText{
			Tagline:          "List all available commands.",
			ShortDescription: `Lists all available commands (and subcommands) and exits.`,
		},
		Options: []cmdkit.Option{
			cmdkit.BoolOption(flagsOptionName, "f", "Show command flags"),
		},
		Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) {
			rootCmd := cmd2outputCmd("ipfs", root)
			rootCmd.showOpts, _ = req.Options[flagsOptionName].(bool)
			err := cmds.EmitOnce(res, &rootCmd)
			if err != nil {
				log.Error(err)
			}
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
	sort.Sort(sort.StringSlice(cmds))
	return cmds
}

// changes here will also need to be applied at
// - ./dag/dag.go
// - ./object/object.go
// - ./files/files.go
// - ./unixfs/unixfs.go
func unwrapOutput(i interface{}) (interface{}, error) {
	var (
		ch <-chan interface{}
		ok bool
	)

	if ch, ok = i.(<-chan interface{}); !ok {
		return nil, e.TypeErr(ch, i)
	}

	return <-ch, nil
}
