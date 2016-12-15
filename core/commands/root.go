package commands

import (
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	dag "github.com/ipfs/go-ipfs/core/commands/dag"
	files "github.com/ipfs/go-ipfs/core/commands/files"
	ocmd "github.com/ipfs/go-ipfs/core/commands/object"
	unixfs "github.com/ipfs/go-ipfs/core/commands/unixfs"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
)

var log = logging.Logger("core/commands")

const (
	ApiOption = "api"
)

var Root = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:  "Global p2p merkle-dag filesystem.",
		Synopsis: "ipfs [--config=<config> | -c] [--debug=<debug> | -D] [--help=<help>] [-h=<h>] [--local=<local> | -L] [--api=<api>] <command> ...",
		AdditionalHelp: `ipfs uses a repository in the local file system. By default, the repo is located
at ~/.ipfs. To change the repo location, set the $IPFS_PATH environment variable:

  export IPFS_PATH=/path/to/ipfsrepo

EXIT STATUS
  The CLI will exit with one of the following values:
    0     Successful execution.
    1     Failed executions.
`,
	},
	Options: []cmds.Option{
		cmds.StringOption("config", "c", "Path to the configuration file to use."),
		cmds.BoolOption("debug", "D", "Operate in debug mode.").Default(false),
		cmds.BoolOption("help", "Show the full command help text.").Default(false),
		cmds.BoolOption("h", "Show a short version of the command help text.").Default(false),
		cmds.BoolOption("local", "L", "Run the command locally, instead of using the daemon.").Default(false),
		cmds.BoolOption("color", "Use colors in console output.").Default(false),
		cmds.StringOption(ApiOption, "Use a specific API instance (defaults to /ip4/127.0.0.1/tcp/5001)"),
	},
}

// commandsDaemonCmd is the "ipfs commands" command for daemon
var CommandsDaemonCmd = CommandsCmd(Root)

var rootSubcommands = []*cmds.CmdInfo{
	{"add", AddCmd, "BASIC COMMANDS"},
	{"cat", CatCmd, "BASIC COMMANDS"},
	{"get", GetCmd, "BASIC COMMANDS"},
	{"ls", LsCmd, "BASIC COMMANDS"},
	{"refs", RefsCmd, "BASIC COMMANDS"},
	{"tour", tourCmd, "BASIC COMMANDS"},
	{"block", BlockCmd, "DATA STRUCTURE COMMANDS"},
	{"object", ocmd.ObjectCmd, "DATA STRUCTURE COMMANDS"},
	{"files", files.FilesCmd, "DATA STRUCTURE COMMANDS"},
	{"mount", MountCmd, "ADVANCED COMMANDS"},
	{"resolve", ResolveCmd, "ADVANCED COMMANDS"},
	{"name", NameCmd, "ADVANCED COMMANDS"},
	{"dns", DNSCmd, "ADVANCED COMMANDS"},
	{"pin", PinCmd, "ADVANCED COMMANDS"},
	{"repo", RepoCmd, "ADVANCED COMMANDS"},
	{"dag", dag.DagCmd, "ADVANCED COMMANDS"},
	{"key", KeyCmd, "ADVANCED COMMANDS"},
	{"pubsub", PubsubCmd, "ADVANCED COMMANDS"},
	{"log", LogCmd, "ADVANCED COMMANDS"},
	{"stats", StatsCmd, "ADVANCED COMMANDS"},
	{"tar", TarCmd, "ADVANCED COMMANDS"},
	{"file", unixfs.UnixFSCmd, "ADVANCED COMMANDS"},
	{"bitswap", BitswapCmd, "ADVANCED COMMANDS"},
	{"id", IDCmd, "NETWORK COMMANDS"},
	{"bootstrap", BootstrapCmd, "NETWORK COMMANDS"},
	{"swarm", SwarmCmd, "NETWORK COMMANDS"},
	{"dht", DhtCmd, "NETWORK COMMANDS"},
	{"ping", PingCmd, "NETWORK COMMANDS"},
	{"diag", DiagCmd, "NETWORK COMMANDS"},
	{"commands", CommandsDaemonCmd, "TOOL COMMANDS"},
	{"config", ConfigCmd, "TOOL COMMANDS"},
	{"version", VersionCmd, "TOOL COMMANDS"},
	{"update", ExternalBinary(), "TOOL COMMANDS"},
}

// RootRO is the readonly version of Root
var RootRO = &cmds.Command{}

var CommandsDaemonROCmd = CommandsCmd(RootRO)

var RefsROCmd = &cmds.Command{}

var rootROSubcommands = []*cmds.CmdInfo{
	{"block",
		&cmds.Command{Subcommands: []*cmds.CmdInfo{{"stat", blockStatCmd, ""}, {"get", blockGetCmd, ""}}},
		""},
	{"cat", CatCmd, ""},
	{"commands", CommandsDaemonROCmd, ""},
	{"dns", DNSCmd, ""},
	{"get", GetCmd, ""},
	{"ls", LsCmd, ""},
	{"name", &cmds.Command{Subcommands: []*cmds.CmdInfo{{"resolve", IpnsCmd, ""}}}, ""},
	{"object",
		&cmds.Command{Subcommands: []*cmds.CmdInfo{
			{"data", ocmd.ObjectDataCmd, ""},
			{"links", ocmd.ObjectLinksCmd, ""},
			{"get", ocmd.ObjectGetCmd, ""},
			{"stat", ocmd.ObjectStatCmd, ""},
			{"patch", ocmd.ObjectPatchCmd, ""},
		}},
		""},
	{"dag", &cmds.Command{Subcommands: []*cmds.CmdInfo{{"get", dag.DagGetCmd, ""}}}, ""},
	{"refs", RefsROCmd, ""},
	{"resolve", ResolveCmd, ""},
	{"version", VersionCmd, ""},
}

func init() {
	Root.ProcessHelp()
	*RootRO = *Root

	// sanitize readonly refs command
	*RefsROCmd = *RefsCmd
	RefsROCmd.Subcommands = []*cmds.CmdInfo{}

	Root.Subcommands = rootSubcommands
	RootRO.Subcommands = rootROSubcommands
}

type MessageOutput struct {
	Message string
}

func MessageTextMarshaler(res cmds.Response) (io.Reader, error) {
	return strings.NewReader(res.Output().(*MessageOutput).Message), nil
}
