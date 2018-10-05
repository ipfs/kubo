package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	lgc "github.com/ipfs/go-ipfs/commands/legacy"
	dag "github.com/ipfs/go-ipfs/core/commands/dag"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	name "github.com/ipfs/go-ipfs/core/commands/name"
	ocmd "github.com/ipfs/go-ipfs/core/commands/object"
	unixfs "github.com/ipfs/go-ipfs/core/commands/unixfs"

	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	"gx/ipfs/QmXTmUCBtDUrzDYVzASogLiNph7EBuYqEgPL7QoHNMzUnz/go-ipfs-cmds"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
)

var log = logging.Logger("core/commands")

var ErrNotOnline = errors.New("this command must be run in online mode. Try running 'ipfs daemon' first")

const (
	ApiOption = "api"
)

var Root = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:  "Global p2p merkle-dag filesystem.",
		Synopsis: "ipfs [--config=<config> | -c] [--debug=<debug> | -D] [--help=<help>] [-h=<h>] [--local=<local> | -L] [--api=<api>] <command> ...",
		Subcommands: `
BASIC COMMANDS
  init          Initialize ipfs local configuration
  add <path>    Add a file to IPFS
  cat <ref>     Show IPFS object data
  get <ref>     Download IPFS objects
  ls <ref>      List links from an object
  refs <ref>    List hashes of links from an object

DATA STRUCTURE COMMANDS
  block         Interact with raw blocks in the datastore
  object        Interact with raw dag nodes
  files         Interact with objects as if they were a unix filesystem
  dag           Interact with IPLD documents (experimental)

ADVANCED COMMANDS
  daemon        Start a long-running daemon process
  mount         Mount an IPFS read-only mountpoint
  resolve       Resolve any type of name
  name          Publish and resolve IPNS names
  key           Create and list IPNS name keypairs
  dns           Resolve DNS links
  pin           Pin objects to local storage
  repo          Manipulate the IPFS repository
  stats         Various operational stats
  p2p           Libp2p stream mounting
  filestore     Manage the filestore (experimental)

NETWORK COMMANDS
  id            Show info about IPFS peers
  bootstrap     Add or remove bootstrap peers
  swarm         Manage connections to the p2p network
  dht           Query the DHT for values or peers
  ping          Measure the latency of a connection
  diag          Print diagnostics

TOOL COMMANDS
  config        Manage configuration
  version       Show ipfs version information
  update        Download and apply go-ipfs updates
  commands      List all available commands
  cid           Convert and discover properties of CIDs

Use 'ipfs <command> --help' to learn more about each command.

ipfs uses a repository in the local file system. By default, the repo is
located at ~/.ipfs. To change the repo location, set the $IPFS_PATH
environment variable:

  export IPFS_PATH=/path/to/ipfsrepo

EXIT STATUS

The CLI will exit with one of the following values:

0     Successful execution.
1     Failed executions.
`,
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("config", "c", "Path to the configuration file to use."),
		cmdkit.BoolOption("debug", "D", "Operate in debug mode."),
		cmdkit.BoolOption("help", "Show the full command help text."),
		cmdkit.BoolOption("h", "Show a short version of the command help text."),
		cmdkit.BoolOption("local", "L", "Run the command locally, instead of using the daemon."),
		cmdkit.StringOption(ApiOption, "Use a specific API instance (defaults to /ip4/127.0.0.1/tcp/5001)"),

		// global options, added to every command
		cmds.OptionEncodingType,
		cmds.OptionStreamChannels,
		cmds.OptionTimeout,
	},
}

// commandsDaemonCmd is the "ipfs commands" command for daemon
var CommandsDaemonCmd = CommandsCmd(Root)

var rootSubcommands = map[string]*cmds.Command{
	"add":       AddCmd,
	"bitswap":   BitswapCmd,
	"block":     BlockCmd,
	"cat":       CatCmd,
	"commands":  CommandsDaemonCmd,
	"files":     FilesCmd,
	"filestore": FileStoreCmd,
	"get":       GetCmd,
	"pubsub":    PubsubCmd,
	"repo":      RepoCmd,
	"stats":     StatsCmd,
	"bootstrap": lgc.NewCommand(BootstrapCmd),
	"config":    lgc.NewCommand(ConfigCmd),
	"dag":       lgc.NewCommand(dag.DagCmd),
	"dht":       lgc.NewCommand(DhtCmd),
	"diag":      lgc.NewCommand(DiagCmd),
	"dns":       lgc.NewCommand(DNSCmd),
	"id":        lgc.NewCommand(IDCmd),
	"key":       KeyCmd,
	"log":       lgc.NewCommand(LogCmd),
	"ls":        lgc.NewCommand(LsCmd),
	"mount":     lgc.NewCommand(MountCmd),
	"name":      name.NameCmd,
	"object":    ocmd.ObjectCmd,
	"pin":       lgc.NewCommand(PinCmd),
	"ping":      lgc.NewCommand(PingCmd),
	"p2p":       lgc.NewCommand(P2PCmd),
	"refs":      lgc.NewCommand(RefsCmd),
	"resolve":   ResolveCmd,
	"swarm":     SwarmCmd,
	"tar":       lgc.NewCommand(TarCmd),
	"file":      lgc.NewCommand(unixfs.UnixFSCmd),
	"update":    lgc.NewCommand(ExternalBinary()),
	"urlstore":  urlStoreCmd,
	"version":   lgc.NewCommand(VersionCmd),
	"shutdown":  daemonShutdownCmd,
	"cid":       CidCmd,
}

func init() {
	Root.ProcessHelp()
	Root.Subcommands = rootSubcommands
}

func RootSubset(allowed []string) (*cmds.Command, error) {
	subset := new(cmds.Command)
	*subset = *Root
	subset.Subcommands = map[string]*cmds.Command{}

	commands := false
	for _, path := range allowed {
		if path == "commands" {
			commands = true
			continue
		}

		pathelems := strings.Split(path, "/")
		in := Root
		out := subset
		for _, elem := range pathelems {
			nextIn, ok := in.Subcommands[elem]
			if !ok {
				return nil, fmt.Errorf("unknown command: %s", path)
			}

			nextOut := new(cmds.Command)
			*nextOut = *nextIn
			nextOut.Subcommands = map[string]*cmds.Command{}

			out.Subcommands[elem] = nextOut
			out = nextOut
			in = nextIn
		}
	}

	if commands {
		subset.Subcommands["commands"] = CommandsCmd(subset)
	}
	return subset, nil
}

type MessageOutput struct {
	Message string
}

func MessageTextMarshaler(res oldcmds.Response) (io.Reader, error) {
	v, err := unwrapOutput(res.Output())
	if err != nil {
		return nil, err
	}

	out, ok := v.(*MessageOutput)
	if !ok {
		return nil, e.TypeErr(out, v)
	}

	return strings.NewReader(out.Message), nil
}
