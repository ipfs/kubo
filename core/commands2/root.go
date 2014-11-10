package commands

import (
	cmds "github.com/jbenet/go-ipfs/commands"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("core/commands")

type TestOutput struct {
	Foo string
	Bar int
}

var Root = &cmds.Command{
	Description: "Global P2P Merkle-DAG filesystem",
	Help: `Basic commands:

    init          Initialize ipfs local configurationx
    add <path>    Add an object to ipfs
    cat <ref>     Show ipfs object data
    ls <ref>      List links from an object

Tool commands:

    config        Manage configuration
    update        Download and apply go-ipfs updates
    version       Show ipfs version information
    commands      List all available commands

Advanced Commands:

    mount         Mount an ipfs read-only mountpoint
    serve         Serve an interface to ipfs
    diag          Print diagnostics

Plumbing commands:

    block         Interact with raw blocks in the datastore
    object        Interact with raw dag nodes


Use "ipfs <command> --help" for more information about a command.
`,

	Options: []cmds.Option{
		cmds.Option{[]string{"config", "c"}, cmds.String,
			"Path to the configuration file to use"},
		cmds.Option{[]string{"debug", "D"}, cmds.Bool,
			"Operate in debug mode"},
		cmds.Option{[]string{"help", "h"}, cmds.Bool,
			"Show the command help text"},
		cmds.Option{[]string{"local", "L"}, cmds.Bool,
			"Run the command locally, instead of using the daemon"},
	},
}

var rootSubcommands = map[string]*cmds.Command{
	"cat":       catCmd,
	"ls":        lsCmd,
	"commands":  commandsCmd,
	"name":      nameCmd,
	"add":       addCmd,
	"log":       logCmd,
	"diag":      diagCmd,
	"pin":       pinCmd,
	"version":   versionCmd,
	"config":    configCmd,
	"bootstrap": bootstrapCmd,
	"mount":     mountCmd,
	"block":     blockCmd,
	"update":    updateCmd,
	"object":    objectCmd,
	"refs":      refsCmd,
}

func init() {
	Root.Subcommands = rootSubcommands
	u.SetLogLevel("core/commands", "info")
}

type MessageOutput struct {
	Message string
}

func MessageTextMarshaller(res cmds.Response) ([]byte, error) {
	return []byte(res.Output().(*MessageOutput).Message), nil
}
