package main

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsCli "github.com/jbenet/go-ipfs/commands/cli"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	core "github.com/jbenet/go-ipfs/core"
	repo "github.com/jbenet/go-ipfs/repo"
	config "github.com/jbenet/go-ipfs/repo/config"
	fsrepo "github.com/jbenet/go-ipfs/repo/fsrepo"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	updates "github.com/jbenet/go-ipfs/updates"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/debugerror"
)

// log is the command logger
var log = eventlog.Logger("cmd/ipfs")

// signal to output help
var errHelpRequested = errors.New("Help Requested")

const (
	cpuProfile  = "ipfs.cpuprof"
	heapProfile = "ipfs.memprof"
	errorFormat = "ERROR: %v\n\n"
)

type cmdInvocation struct {
	path []string
	cmd  *cmds.Command
	req  cmds.Request
	node *core.IpfsNode
}

// main roadmap:
// - parse the commandline to get a cmdInvocation
// - if user requests, help, print it and exit.
// - run the command invocation
// - output the response
// - if anything fails, print error, maybe with help
func main() {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(3) // FIXME rm arbitrary choice for n
	ctx := eventlog.ContextWithLoggable(context.Background(), eventlog.Uuid("session"))
	var err error
	var invoc cmdInvocation
	defer invoc.close()

	// we'll call this local helper to output errors.
	// this is so we control how to print errors in one place.
	printErr := func(err error) {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}

	// this is a local helper to print out help text.
	// there's some considerations that this makes easier.
	printHelp := func(long bool, w io.Writer) {
		helpFunc := cmdsCli.ShortHelp
		if long {
			helpFunc = cmdsCli.LongHelp
		}

		helpFunc("ipfs", Root, invoc.path, w)
	}

	// parse the commandline into a command invocation
	parseErr := invoc.Parse(ctx, os.Args[1:])

	// BEFORE handling the parse error, if we have enough information
	// AND the user requested help, print it out and exit
	if invoc.req != nil {
		longH, shortH, err := invoc.requestedHelp()
		if err != nil {
			printErr(err)
			os.Exit(1)
		}
		if longH || shortH {
			printHelp(longH, os.Stdout)
			os.Exit(0)
		}
	}

	// here we handle the cases where
	// - commands with no Run func are invoked directly.
	// - the main command is invoked.
	if invoc.cmd == nil || invoc.cmd.Run == nil {
		printHelp(false, os.Stdout)
		os.Exit(0)
	}

	// ok now handle parse error (which means cli input was wrong,
	// e.g. incorrect number of args, or nonexistent subcommand)
	if parseErr != nil {
		printErr(parseErr)

		// this was a user error, print help.
		if invoc.cmd != nil {
			// we need a newline space.
			fmt.Fprintf(os.Stderr, "\n")
			printHelp(false, os.Stderr)
		}
		os.Exit(1)
	}

	// ok, finally, run the command invocation.
	output, err := invoc.Run(ctx)
	if err != nil {
		printErr(err)

		// if this error was a client error, print short help too.
		if isClientError(err) {
			printHelp(false, os.Stderr)
		}
		os.Exit(1)
	}

	// everything went better than expected :)
	io.Copy(os.Stdout, output)
}

func (i *cmdInvocation) Run(ctx context.Context) (output io.Reader, err error) {
	// setup our global interrupt handler.
	i.setupInterruptHandler()

	// check if user wants to debug. option OR env var.
	debug, _, err := i.req.Option("debug").Bool()
	if err != nil {
		return nil, err
	}
	if debug || u.GetenvBool("DEBUG") || os.Getenv("IPFS_LOGGING") == "debug" {
		u.Debug = true
		u.SetDebugLogging()
	}

	// if debugging, let's profile.
	// TODO maybe change this to its own option... profiling makes it slower.
	if u.Debug {
		stopProfilingFunc, err := startProfiling()
		if err != nil {
			return nil, err
		}
		defer stopProfilingFunc() // to be executed as late as possible
	}

	res, err := callCommand(ctx, i.req, Root, i.cmd)
	if err != nil {
		return nil, err
	}

	if err := res.Error(); err != nil {
		return nil, err
	}

	return res.Reader()
}

func (i *cmdInvocation) constructNodeFunc(ctx context.Context) func() (*core.IpfsNode, error) {
	return func() (*core.IpfsNode, error) {
		if i.req == nil {
			return nil, errors.New("constructing node without a request")
		}

		cmdctx := i.req.Context()
		if cmdctx == nil {
			return nil, errors.New("constructing node without a request context")
		}

		r := fsrepo.At(i.req.Context().ConfigRoot)
		if err := r.Open(); err != nil { // repo is owned by the node
			return nil, err
		}

		// ok everything is good. set it on the invocation (for ownership)
		// and return it.
		n, err := core.NewIPFSNode(ctx, core.Standard(r, cmdctx.Online))
		if err != nil {
			return nil, err
		}
		i.node = n
		return i.node, nil
	}
}

func (i *cmdInvocation) close() {
	// let's not forget teardown. If a node was initialized, we must close it.
	// Note that this means the underlying req.Context().Node variable is exposed.
	// this is gross, and should be changed when we extract out the exec Context.
	if i.node != nil {
		log.Info("Shutting down node...")
		i.node.Close()
	}
}

func (i *cmdInvocation) Parse(ctx context.Context, args []string) error {
	var err error

	i.req, i.cmd, i.path, err = cmdsCli.Parse(args, os.Stdin, Root)
	if err != nil {
		return err
	}

	configPath, err := getConfigRoot(i.req)
	if err != nil {
		return err
	}
	log.Debugf("config path is %s", configPath)

	// this sets up the function that will initialize the config lazily.
	cmdctx := i.req.Context()
	cmdctx.ConfigRoot = configPath
	cmdctx.LoadConfig = loadConfig
	// this sets up the function that will initialize the node
	// this is so that we can construct the node lazily.
	cmdctx.ConstructNode = i.constructNodeFunc(ctx)

	// if no encoding was specified by user, default to plaintext encoding
	// (if command doesn't support plaintext, use JSON instead)
	if !i.req.Option("encoding").Found() {
		if i.req.Command().Marshalers != nil && i.req.Command().Marshalers[cmds.Text] != nil {
			i.req.SetOption("encoding", cmds.Text)
		} else {
			i.req.SetOption("encoding", cmds.JSON)
		}
	}

	return nil
}

func (i *cmdInvocation) requestedHelp() (short bool, long bool, err error) {
	longHelp, _, err := i.req.Option("help").Bool()
	if err != nil {
		return false, false, err
	}
	shortHelp, _, err := i.req.Option("h").Bool()
	if err != nil {
		return false, false, err
	}
	return longHelp, shortHelp, nil
}

func callPreCommandHooks(ctx context.Context, details cmdDetails, req cmds.Request, root *cmds.Command) error {

	log.Event(ctx, "callPreCommandHooks", &details)
	log.Debug("Calling pre-command hooks...")

	// some hooks only run when the command is executed locally
	daemon, err := commandShouldRunOnDaemon(details, req, root)
	if err != nil {
		return err
	}

	// check for updates when 1) commands is going to be run locally, 2) the
	// command does not initialize the config, and 3) the command does not
	// pre-empt updates
	if !daemon && details.usesConfigAsInput() && details.doesNotPreemptAutoUpdate() {

		log.Debug("Calling hook: Check for updates")

		cfg, err := req.Context().GetConfig()
		if err != nil {
			return err
		}
		// Check for updates and potentially install one.
		if err := updates.CliCheckForUpdates(cfg, req.Context().ConfigRoot); err != nil {
			return err
		}
	}

	// When the upcoming command may use the config and repo, we know it's safe
	// for the log config hook to touch the config/repo
	if repo.IsInitialized(req.Context().ConfigRoot) {
		log.Debug("Calling hook: Configure Event Logger")
		cfg, err := req.Context().GetConfig()
		if err != nil {
			return err
		}
		repo.ConfigureEventLogger(cfg.Logs)
	}

	return nil
}

func callCommand(ctx context.Context, req cmds.Request, root *cmds.Command, cmd *cmds.Command) (cmds.Response, error) {
	var res cmds.Response

	details, err := commandDetails(req.Path(), root)
	if err != nil {
		return nil, err
	}

	log.Info("looking for running daemon...")
	useDaemon, err := commandShouldRunOnDaemon(*details, req, root)
	if err != nil {
		return nil, err
	}

	err = callPreCommandHooks(ctx, *details, req, root)
	if err != nil {
		return nil, err
	}

	if useDaemon {

		cfg, err := req.Context().GetConfig()
		if err != nil {
			return nil, err
		}

		addr, err := ma.NewMultiaddr(cfg.Addresses.API)
		if err != nil {
			return nil, err
		}

		log.Infof("Executing command on daemon running at %s", addr)
		_, host, err := manet.DialArgs(addr)
		if err != nil {
			return nil, err
		}

		client := cmdsHttp.NewClient(host)

		res, err = client.Send(req)
		if err != nil {
			return nil, err
		}

	} else {
		log.Info("Executing command locally")

		// Okay!!!!! NOW we can call the command.
		res = root.Call(req)

	}

	if cmd.PostRun != nil {
		cmd.PostRun(res)
	}

	return res, nil
}

// commandDetails returns a command's details for the command given by |path|
// within the |root| command tree.
//
// Returns an error if the command is not found in the Command tree.
func commandDetails(path []string, root *cmds.Command) (*cmdDetails, error) {
	var details cmdDetails
	// find the last command in path that has a cmdDetailsMap entry
	cmd := root
	for _, cmp := range path {
		var found bool
		cmd, found = cmd.Subcommands[cmp]
		if !found {
			return nil, debugerror.Errorf("subcommand %s should be in root", cmp)
		}

		if cmdDetails, found := cmdDetailsMap[cmd]; found {
			details = cmdDetails
		}
	}
	return &details, nil
}

// commandShouldRunOnDaemon determines, from commmand details, whether a
// command ought to be executed on an IPFS daemon.
//
// It returns true if the command should be executed on a daemon and false if
// it should be executed on a client. It returns an error if the command must
// NOT be executed on either.
func commandShouldRunOnDaemon(details cmdDetails, req cmds.Request, root *cmds.Command) (bool, error) {
	path := req.Path()
	// root command.
	if len(path) < 1 {
		return false, nil
	}

	if details.cannotRunOnClient && details.cannotRunOnDaemon {
		return false, fmt.Errorf("command disabled: %s", path[0])
	}

	if details.doesNotUseRepo && details.canRunOnClient() {
		return false, nil
	}

	// at this point need to know whether daemon is running. we defer
	// to this point so that some commands dont open files unnecessarily.
	daemonLocked := fsrepo.LockedByOtherProcess(req.Context().ConfigRoot)

	if daemonLocked {

		log.Info("a daemon is running...")

		if details.cannotRunOnDaemon {
			e := "ipfs daemon is running. please stop it to run this command"
			return false, cmds.ClientError(e)
		}

		return true, nil
	}

	if details.cannotRunOnClient {
		return false, cmds.ClientError("must run on the ipfs daemon")
	}

	return false, nil
}

func isClientError(err error) bool {

	// Somewhat suprisingly, the pointer cast fails to recognize commands.Error
	// passed as values, so we check both.

	// cast to cmds.Error
	switch e := err.(type) {
	case *cmds.Error:
		return e.Code == cmds.ErrClient
	case cmds.Error:
		return e.Code == cmds.ErrClient
	}
	return false
}

func getConfigRoot(req cmds.Request) (string, error) {
	configOpt, found, err := req.Option("config").String()
	if err != nil {
		return "", err
	}
	if found && configOpt != "" {
		return configOpt, nil
	}

	configPath, err := config.PathRoot()
	if err != nil {
		return "", err
	}
	return configPath, nil
}

func loadConfig(path string) (*config.Config, error) {
	return fsrepo.ConfigAt(path)
}

// startProfiling begins CPU profiling and returns a `stop` function to be
// executed as late as possible. The stop function captures the memprofile.
func startProfiling() (func(), error) {

	// start CPU profiling as early as possible
	ofi, err := os.Create(cpuProfile)
	if err != nil {
		return nil, err
	}
	pprof.StartCPUProfile(ofi)

	stopProfiling := func() {
		pprof.StopCPUProfile()
		defer ofi.Close() // captured by the closure
		err := writeHeapProfileToFile()
		if err != nil {
			log.Critical(err)
		}
	}
	return stopProfiling, nil
}

func writeHeapProfileToFile() error {
	mprof, err := os.Create(heapProfile)
	if err != nil {
		return err
	}
	defer mprof.Close() // _after_ writing the heap profile
	return pprof.WriteHeapProfile(mprof)
}

// listen for and handle SIGTERM
func (i *cmdInvocation) setupInterruptHandler() {

	ctx := i.req.Context()
	sig := allInterruptSignals()

	go func() {
		// first time, try to shut down.

		// loop because we may be
		for count := 0; ; count++ {
			<-sig

			n, err := ctx.GetNode()
			if err != nil {
				log.Error(err)
				log.Critical("Received interrupt signal, terminating...")
				os.Exit(-1)
			}

			switch count {
			case 0:
				log.Critical("Received interrupt signal, shutting down...")
				go func() {
					n.Close()
					log.Info("Gracefully shut down.")
				}()

			default:
				log.Critical("Received another interrupt before graceful shutdown, terminating...")
				os.Exit(-1)
			}
		}
	}()
}

func allInterruptSignals() chan os.Signal {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT,
		syscall.SIGTERM)
	return sigc
}
