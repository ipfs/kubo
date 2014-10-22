package commands

import (
	cmds "github.com/jbenet/go-ipfs/commands"
	"strings"
)

type TestOutput struct {
	Foo string
	Bar int
}

var Root = &cmds.Command{
	Options: []cmds.Option{
		cmds.Option{[]string{"config", "c"}, cmds.String},
		cmds.Option{[]string{"debug", "D"}, cmds.Bool},
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
    version       Show ipfs version information.
    commands      List all available commands.

Advanced Commands:

    mount         Mount an ipfs read-only mountpoint.
    serve         Serve an interface to ipfs.
    net-diag      Print network diagnostic.

Use "ipfs help <command>" for more information about a command.
`,
	Subcommands: map[string]*cmds.Command{
		"beep": &cmds.Command{
			Run: func(req cmds.Request, res cmds.Response) {
				v := TestOutput{"hello, world", 1337}
				res.SetValue(v)
			},
		},
		"boop": &cmds.Command{
			Run: func(req cmds.Request, res cmds.Response) {
				v := strings.NewReader("hello, world")
				res.SetValue(v)
			},
		},
	},
}
