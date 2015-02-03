package commands

import (
	cmds "github.com/jbenet/go-ipfs/commands"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"

	add "github.com/jbenet/go-ipfs/core/commands/add"
	block "github.com/jbenet/go-ipfs/core/commands/block"
	bootstrap "github.com/jbenet/go-ipfs/core/commands/bootstrap"
	cat "github.com/jbenet/go-ipfs/core/commands/cat"
	corecommands "github.com/jbenet/go-ipfs/core/commands/commands"
	config "github.com/jbenet/go-ipfs/core/commands/config"
	dag "github.com/jbenet/go-ipfs/core/commands/dag"
	dht "github.com/jbenet/go-ipfs/core/commands/dht"
	diag "github.com/jbenet/go-ipfs/core/commands/diag"
	get "github.com/jbenet/go-ipfs/core/commands/get"
	id "github.com/jbenet/go-ipfs/core/commands/id"
	cmdlog "github.com/jbenet/go-ipfs/core/commands/log"
	ls "github.com/jbenet/go-ipfs/core/commands/ls"
	mount "github.com/jbenet/go-ipfs/core/commands/mount"
	name "github.com/jbenet/go-ipfs/core/commands/name"
	pin "github.com/jbenet/go-ipfs/core/commands/pin"
	ping "github.com/jbenet/go-ipfs/core/commands/ping"
	refs "github.com/jbenet/go-ipfs/core/commands/refs"
	repo "github.com/jbenet/go-ipfs/core/commands/repo"
	swarm "github.com/jbenet/go-ipfs/core/commands/swarm"
	update "github.com/jbenet/go-ipfs/core/commands/update"
	version "github.com/jbenet/go-ipfs/core/commands/version"
)

var log = eventlog.Logger("core/cmds")

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
    add  <path>   Add an object to ipfs
    cat  <ref>    Show ipfs object data
    ls   <ref>    List links from an object
    refs <ref>    List hashes of links from an object
    get  <ref>    Download ipfs objects

Node commands:

    id            Show info about ipfs peers
    pin           Pin objects to local storage
    name          Publish or resolve IPNS names
    daemon        Start a long-running daemon process
    mount         Mount an ipfs read-only mountpoint

Network commands:

    bootstrap     Add or remove bootstrap peers
    swarm         Manage connections to the p2p network
    ping          Measure the latency of a connection
    diag          Print diagnostics

Data structure commands:

    block         Manipulate raw data blocks in the datastore
    dag           Manipulate raw merkle dag nodes
    file          Manipulate unixfs files (wip)

Tool commands:

    log           Change the logging level
    config        Manage configuration
    update        Download and apply go-ipfs updates (wip)
    version       Show ipfs version information
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
var CommandsDaemonCmd = corecommands.CommandsCmd(Root)

var rootSubcommands = map[string]*cmds.Command{
	"add":       add.AddCmd,
	"block":     block.BlockCmd,
	"bootstrap": bootstrap.BootstrapCmd,
	"cat":       cat.CatCmd,
	"commands":  CommandsDaemonCmd,
	"config":    config.ConfigCmd,
	"dht":       dht.DhtCmd,
	"diag":      diag.DiagCmd,
	"get":       get.GetCmd,
	"id":        id.IDCmd,
	"log":       cmdlog.LogCmd,
	"ls":        ls.LsCmd,
	"mount":     mount.MountCmd,
	"name":      name.NameCmd,
	"dag":       dag.DagCmd,
	"pin":       pin.PinCmd,
	"ping":      ping.PingCmd,
	"refs":      refs.RefsCmd,
	"repo":      repo.RepoCmd,
	"swarm":     swarm.SwarmCmd,
	"update":    update.UpdateCmd,
	"version":   version.VersionCmd,
}

func init() {
	Root.Subcommands = rootSubcommands
}
