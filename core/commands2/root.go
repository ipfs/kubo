package commands

import (
	"fmt"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("core/commands")

type TestOutput struct {
	Foo string
	Bar int
}

var Root = &cmds.Command{
	Options: []cmds.Option{
		cmds.Option{[]string{"config", "c"}, cmds.String},
		cmds.Option{[]string{"debug", "D"}, cmds.Bool},
		cmds.Option{[]string{"help", "h"}, cmds.Bool},
		cmds.Option{[]string{"local", "L"}, cmds.Bool},
	},
	Help: `ipfs - global versioned p2p merkledag file system

Basic commands:

    init          Initialize ipfs local configuration.
    add <path>    Add an object to ipfs.
    cat <ref>     Show ipfs object data.
    ls <ref>      List links from an object.
    refs <ref>    List link hashes from an object.

Tool commands:

    config        Manage configuration.
    update        Download and apply go-ipfs updates.
    version       Show ipfs version information.
    commands      List all available commands.

Advanced Commands:

    mount         Mount an ipfs read-only mountpoint.
    serve         Serve an interface to ipfs.
    net-diag      Print network diagnostic

Plumbing commands:

    block         Interact with raw blocks in the datastore
    object        Interact with raw dag nodes


Use "ipfs help <command>" for more information about a command.
`,
}

var rootSubcommands = map[string]*cmds.Command{
	"cat":      catCmd,
	"ls":       lsCmd,
	"commands": commandsCmd,
	"publish":  publishCmd,
	"add":      addCmd,
	"log":      logCmd,

	// test subcommands
	// TODO: remove these when we don't need them anymore
	"beep": &cmds.Command{
		Run: func(res cmds.Response, req cmds.Request) {
			v := &TestOutput{"hello, world", 1337}
			log.Info("beep")
			res.SetValue(v)
		},
		Format: func(res cmds.Response) (string, error) {
			v := res.Value().(*TestOutput)
			s := fmt.Sprintf("Foo: %s\n", v.Foo)
			s += fmt.Sprintf("Bar: %v\n", v.Bar)
			return s, nil
		},
		Type: &TestOutput{},
	},
	"boop": &cmds.Command{
		Run: func(res cmds.Response, req cmds.Request) {
			v := strings.NewReader("hello, world")
			res.SetValue(v)
		},
	},
	"warp": &cmds.Command{
		Options: []cmds.Option{
			cmds.Option{[]string{"power", "p"}, cmds.Float},
		},
		Run: func(res cmds.Response, req cmds.Request) {
			threshold := 1.21

			if power, found := req.Option("power"); found && power.(float64) >= threshold {
				res.SetValue(struct {
					Status string
					Power  float64
				}{"Flux capacitor activated!", power.(float64)})

			} else {
				err := fmt.Errorf("Insufficient power (%v jiggawatts required)", threshold)
				res.SetError(err, cmds.ErrClient)
			}
		},
	},
	"args": &cmds.Command{
		Run: func(res cmds.Response, req cmds.Request) {
			res.SetValue(req.Arguments())
		},
	},
}

func init() {
	Root.Subcommands = rootSubcommands
	u.SetLogLevel("core/commands", "info")
}

type MessageOutput struct {
	Message string
}

func MessageFormatter(res cmds.Response) (string, error) {
	return res.Value().(*MessageOutput).Message, nil
}
