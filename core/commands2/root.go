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
	Helptext: cmds.HelpText{
		Tagline: "Global P2P Merkle-DAG filesystem",
		ShortDescription: `
Basic commands:

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
`,
	},
	Options: []cmds.Option{
		cmds.StringOption("config", "c", "Path to the configuration file to use"),
		cmds.BoolOption("debug", "D", "Operate in debug mode"),
		cmds.BoolOption("help", "Show the full command help text"),
		cmds.BoolOption("h", "Show a short version of the command help text"),
		cmds.BoolOption("local", "L", "Run the command locally, instead of using the daemon"),
	},
}

var rootSubcommands = map[string]*cmds.Command{
	"cat":       catCmd,
	"ls":        lsCmd,
	"commands":  CommandsCmd(Root),
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
