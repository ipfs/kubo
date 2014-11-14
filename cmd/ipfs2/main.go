package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/pprof"

	logging "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-logging"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsCli "github.com/jbenet/go-ipfs/commands/cli"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	"github.com/jbenet/go-ipfs/config"
	"github.com/jbenet/go-ipfs/core"
	daemon "github.com/jbenet/go-ipfs/daemon2"
	u "github.com/jbenet/go-ipfs/util"
)

// log is the command logger
var log = u.Logger("cmd/ipfs")

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
}

// main roadmap:
// - parse the commandline to get a cmdInvocation
// - if user requests, help, print it and exit.
// - run the command invocation
// - output the response
// - if anything fails, print error, maybe with help
func main() {
	var invoc cmdInvocation
	var err error

	// we'll call this local helper to output errors.
	// this is so we control how to print errors in one place.
	printErr := func(err error) {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}

	// this is a local helper to print out help text.
	// there's some considerations that this makes easier.
	printHelp := func(long bool) {
		helpFunc := cmdsCli.ShortHelp
		if long {
			helpFunc = cmdsCli.LongHelp
		}

		helpFunc("ipfs", Root, invoc.path, os.Stderr)
	}

	// parse the commandline into a command invocation
	parseErr := invoc.Parse(os.Args[1:])

	// BEFORE handling the parse error, if we have enough information
	// AND the user requested help, print it out and exit
	if invoc.req != nil {
		longH, shortH, err := invoc.requestedHelp()
		if err != nil {
			printErr(err)
			os.Exit(1)
		}
		if longH || shortH {
			printHelp(longH)
			os.Exit(0)
		}
	}

	// here we handle the cases where
	// - commands with no Run func are invoked directly.
	// - the main command is invoked.
	if invoc.cmd == nil || invoc.cmd.Run == nil {
		printHelp(false)
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
			printHelp(false)
		}
		os.Exit(1)
	}

	// ok, finally, run the command invocation.
	output, err := invoc.Run()
	if err != nil {
		printErr(err)

		// if this error was a client error, print short help too.
		if isClientError(err) {
			printHelp(false)
		}
		os.Exit(1)
	}

	// everything went better than expected :)
	io.Copy(os.Stdout, output)
}

func (i *cmdInvocation) Run() (output io.Reader, err error) {
	handleInterrupt()

	// check if user wants to debug. option OR env var.
	debug, _, err := i.req.Option("debug").Bool()
	if err != nil {
		return nil, err
	}
	if debug || u.GetenvBool("DEBUG") {
		u.Debug = true
		u.SetAllLoggers(logging.DEBUG)
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

	res, err := callCommand(i.req, Root)
	if err != nil {
		return nil, err
	}

	if err := res.Error(); err != nil {
		return nil, err
	}

	return res.Reader()
}

func (i *cmdInvocation) Parse(args []string) error {
	var err error

	i.req, i.cmd, i.path, err = cmdsCli.Parse(args, Root)
	if err != nil {
		return err
	}

	configPath, err := getConfigRoot(i.req)
	if err != nil {
		return err
	}

	// this sets up the function that will initialize the config lazily.
	ctx := i.req.Context()
	ctx.ConfigRoot = configPath
	ctx.LoadConfig = loadConfig

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

func callCommand(req cmds.Request, root *cmds.Command) (cmds.Response, error) {
	var res cmds.Response

	useDaemon, err := commandShouldRunOnDaemon(req, root)
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

		// this sets up the function that will initialize the node
		// this is so that we can construct the node lazily.
		ctx := req.Context()
		ctx.ConstructNode = func() (*core.IpfsNode, error) {
			cfg, err := ctx.GetConfig()
			if err != nil {
				return nil, err
			}
			return core.NewIpfsNode(cfg, false)
		}

		// Okay!!!!! NOW we can call the command.
		res = root.Call(req)

		// let's not forget teardown. If a node was initialized, we must close it.
		// Note that this means the underlying req.Context().Node variable is exposed.
		// this is gross, and should be changed when we extract out the exec Context.
		node := req.Context().NodeWithoutConstructing()
		if node != nil {
			node.Close()
		}
	}
	return res, nil
}

func commandShouldRunOnDaemon(req cmds.Request, root *cmds.Command) (bool, error) {
	path := req.Path()
	// root command.
	if len(path) < 1 {
		return false, nil
	}

	cmd, found := root.Subcommands[path[0]]
	if !found {
		return false, fmt.Errorf("subcommand %s should be in root", path[0])
	}

	details, found := cmdDetailsMap[cmd]
	if !found {
		details = cmdDetails{} // defaults
	}
	log.Debugf("cmd perms for +%v: %s", path, details.String())

	if details.cannotRunOnClient && details.cannotRunOnDaemon {
		return false, fmt.Errorf("command disabled: %s", path[0])
	}

	if details.doesNotUseRepo && details.canRunOnClient() {
		return false, nil
	}

	// at this point need to know whether daemon is running. we defer
	// to this point so that some commands dont open files unnecessarily.
	daemonLocked := daemon.Locked(req.Context().ConfigRoot)
	log.Info("Daemon is running.")

	if daemonLocked {

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
	configFile, err := config.Filename(path)
	if err != nil {
		return nil, err
	}

	return config.Load(configFile)
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
func handleInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		for _ = range c {
			log.Info("Received interrupt signal, terminating...")
			os.Exit(0)
		}
	}()
}
