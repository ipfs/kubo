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
		Tagline: "global p2p merkle-dag filesystem",
		Synopsis: `
ipfs [<flags>] <command> [<arg>] ...
`,
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

    daemon        Start a long-running daemon process
    mount         Mount an ipfs read-only mountpoint
    serve         Serve an interface to ipfs
    diag          Print diagnostics

Plumbing commands:

    block         Interact with raw blocks in the datastore
    object        Interact with raw dag nodes

Use 'ipfs <command> --help' to learn more about each command.
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

// commandsDaemonCmd is the "ipfs commands" command for daemon
var CommandsDaemonCmd = CommandsCmd(Root)

var rootSubcommands = map[string]*cmds.Command{
	"cat":       catCmd,
	"ls":        lsCmd,
	"commands":  CommandsDaemonCmd,
	"name":      nameCmd,
	"add":       addCmd,
	"log":       LogCmd,
	"diag":      DiagCmd,
	"pin":       pinCmd,
	"version":   VersionCmd,
	"config":    configCmd,
	"bootstrap": bootstrapCmd,
	"mount":     mountCmd,
	"block":     blockCmd,
	"update":    UpdateCmd,
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

func MessageTextMarshaler(res cmds.Response) ([]byte, error) {
	return []byte(res.Output().(*MessageOutput).Message), nil
}
