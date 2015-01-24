package commands

import (
	"io"
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
	Helptext: cmds.HelpText{
		Tagline: "global p2p merkle-dag filesystem",
		Synopsis: `
ipfs [<flags>] <command> [<arg>] ...
`,
		ShortDescription: `
Basic commands:

    init          Initialize ipfs local configuration
    add <path>    Add an object to ipfs
    cat <ref>     Show ipfs object data
    ls <ref>      List links from an object
    refs <ref>    List hashes of links from an object

Tool commands:

    config        Manage configuration
    update        Download and apply go-ipfs updates
    version       Show ipfs version information
    commands      List all available commands
    id            Show info about ipfs peers
    pin           Pin objects to local storage
    name          Publish or resolve IPNS names
    log           Change the logging level

Advanced Commands:

    daemon        Start a long-running daemon process
    mount         Mount an ipfs read-only mountpoint
    diag          Print diagnostics

Network commands:

    swarm         Manage connections to the p2p network
    bootstrap     Add or remove bootstrap peers
    ping          Measure the latency of a connection

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
	"add":       AddCmd,
	"block":     BlockCmd,
	"bootstrap": BootstrapCmd,
	"cat":       CatCmd,
	"commands":  CommandsDaemonCmd,
	"config":    ConfigCmd,
	"dht":       DhtCmd,
	"diag":      DiagCmd,
	"id":        IDCmd,
	"log":       LogCmd,
	"ls":        LsCmd,
	"mount":     MountCmd,
	"name":      NameCmd,
	"object":    ObjectCmd,
	"pin":       PinCmd,
	"ping":      PingCmd,
	"refs":      RefsCmd,
	"repo":      RepoCmd,
	"swarm":     SwarmCmd,
	"update":    UpdateCmd,
	"version":   VersionCmd,
}

func init() {
	Root.Subcommands = rootSubcommands
}

type MessageOutput struct {
	Message string
}

func MessageTextMarshaler(res cmds.Response) (io.Reader, error) {
	return strings.NewReader(res.Output().(*MessageOutput).Message), nil
}
