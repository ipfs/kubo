package commands

import (
	"errors"

	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	dag "github.com/ipfs/kubo/core/commands/dag"
	name "github.com/ipfs/kubo/core/commands/name"
	ocmd "github.com/ipfs/kubo/core/commands/object"
	"github.com/ipfs/kubo/core/commands/pin"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("core/commands")

var (
	ErrNotOnline       = errors.New("this command must be run in online mode. Try running 'ipfs daemon' first")
	ErrSelfUnsupported = errors.New("finding your own node in the DHT is currently not supported")
)

const (
	RepoDirOption    = "repo-dir"
	ConfigFileOption = "config-file"
	ConfigOption     = "config"
	DebugOption      = "debug"
	LocalOption      = "local" // DEPRECATED: use OfflineOption
	OfflineOption    = "offline"
	ApiOption        = "api"      //nolint
	ApiAuthOption    = "api-auth" //nolint
)

var Root = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:  "Global p2p merkle-dag filesystem.",
		Synopsis: "ipfs [--config=<config> | -c] [--debug | -D] [--help] [-h] [--api=<api>] [--offline] [--cid-base=<base>] [--upgrade-cidv0-in-output] [--encoding=<encoding> | --enc] [--timeout=<timeout>] <command> ...",
		Subcommands: `
BASIC COMMANDS
  init          Initialize local IPFS configuration
  add <path>    Add a file to IPFS
  cat <ref>     Show IPFS object data
  get <ref>     Download IPFS objects
  ls <ref>      List links from an object
  refs <ref>    List hashes of links from an object

DATA STRUCTURE COMMANDS
  dag           Interact with IPLD DAG nodes
  files         Interact with files as if they were a unix filesystem
  block         Interact with raw blocks in the datastore

TEXT ENCODING COMMANDS
  cid           Convert and discover properties of CIDs
  multibase     Encode and decode data with Multibase format

ADVANCED COMMANDS
  daemon        Start a long-running daemon process
  shutdown      Shut down the daemon process
  resolve       Resolve any type of content path
  name          Publish and resolve IPNS names
  key           Create and list IPNS name keypairs
  pin           Pin objects to local storage
  repo          Manipulate the IPFS repository
  stats         Various operational stats
  p2p           Libp2p stream mounting (experimental)
  filestore     Manage the filestore (experimental)
  mount         Mount an IPFS read-only mount point (experimental)

NETWORK COMMANDS
  id            Show info about IPFS peers
  bootstrap     Add or remove bootstrap peers
  swarm         Manage connections to the p2p network
  dht           Query the DHT for values or peers
  routing       Issue routing commands
  ping          Measure the latency of a connection
  bitswap       Inspect bitswap state
  pubsub        Send and receive messages via pubsub

TOOL COMMANDS
  config        Manage configuration
  version       Show IPFS version information
  diag          Generate diagnostic reports
  update        Download and apply go-ipfs updates
  commands      List all available commands
  log           Manage and show logs of running daemon

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
	Options: []cmds.Option{
		cmds.StringOption(RepoDirOption, "Path to the repository directory to use."),
		cmds.StringOption(ConfigFileOption, "Path to the configuration file to use."),
		cmds.StringOption(ConfigOption, "c", "[DEPRECATED] Path to the configuration file to use."),
		cmds.BoolOption(DebugOption, "D", "Operate in debug mode."),
		cmds.BoolOption(cmds.OptLongHelp, "Show the full command help text."),
		cmds.BoolOption(cmds.OptShortHelp, "Show a short version of the command help text."),
		cmds.BoolOption(LocalOption, "L", "Run the command locally, instead of using the daemon. DEPRECATED: use --offline."),
		cmds.BoolOption(OfflineOption, "Run the command offline."),
		cmds.StringOption(ApiOption, "Use a specific API instance (defaults to /ip4/127.0.0.1/tcp/5001)"),
		cmds.StringOption(ApiAuthOption, "Optional RPC API authorization secret (defined as AuthSecret in API.Authorizations config)"),

		// global options, added to every command
		cmdenv.OptionCidBase,
		cmdenv.OptionUpgradeCidV0InOutput,

		cmds.OptionEncodingType,
		cmds.OptionStreamChannels,
		cmds.OptionTimeout,
	},
}

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
	"bootstrap": BootstrapCmd,
	"config":    ConfigCmd,
	"dag":       dag.DagCmd,
	"dht":       DhtCmd,
	"routing":   RoutingCmd,
	"diag":      DiagCmd,
	"id":        IDCmd,
	"key":       KeyCmd,
	"log":       LogCmd,
	"ls":        LsCmd,
	"mount":     MountCmd,
	"name":      name.NameCmd,
	"object":    ocmd.ObjectCmd,
	"pin":       pin.PinCmd,
	"ping":      PingCmd,
	"p2p":       P2PCmd,
	"refs":      RefsCmd,
	"resolve":   ResolveCmd,
	"swarm":     SwarmCmd,
	"update":    ExternalBinary("Please see https://github.com/ipfs/ipfs-update/blob/master/README.md#install for installation instructions."),
	"version":   VersionCmd,
	"shutdown":  daemonShutdownCmd,
	"cid":       CidCmd,
	"multibase": MbaseCmd,
}

func init() {
	Root.ProcessHelp()
	Root.Subcommands = rootSubcommands
}

type MessageOutput struct {
	Message string
}
