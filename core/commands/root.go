package commands

import (
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	evlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
)

var log = evlog.Logger("core/commands")

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
BASIC COMMANDS

    init          Initialize ipfs local configuration
    add <path>    Add an object to ipfs
    cat <ref>     Show ipfs object data
    get <ref>     Download ipfs objects
    ls <ref>      List links from an object
    refs <ref>    List hashes of links from an object

DATA STRUCTURE COMMANDS

    block         Interact with raw blocks in the datastore
    object        Interact with raw dag nodes

ADVANCED COMMANDS

    daemon        Start a long-running daemon process
    mount         Mount an ipfs read-only mountpoint
    name          Publish or resolve IPNS names
    dns           Resolve DNS links
    pin           Pin objects to local storage
    repo gc       Garbage collect unpinned objects

NETWORK COMMANDS

    id            Show info about ipfs peers
    bootstrap     Add or remove bootstrap peers
    swarm         Manage connections to the p2p network
    dht           Query the dht for values or peers
    ping          Measure the latency of a connection
    diag          Print diagnostics

TOOL COMMANDS

    config        Manage configuration
    version       Show ipfs version information
    update        Download and apply go-ipfs updates
    commands      List all available commands

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
	"dns":       DNSCmd,
	"get":       GetCmd,
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
	"stats":     StatsCmd,
	"swarm":     SwarmCmd,
	"update":    UpdateCmd,
	"version":   VersionCmd,
	"bitswap":   BitswapCmd,
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
