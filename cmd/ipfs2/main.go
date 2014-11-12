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
	commands "github.com/jbenet/go-ipfs/core/commands2"
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

func main() {
	err := run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	handleInterrupt()

	args := os.Args[1:]
	req, root, err := createRequest(args)
	if err != nil {
		// when the error is errOutputHelp, just exit gracefully.
		if err == errHelpRequested {
			return nil
		}
		return err
	}

	debug, _, err := req.Option("debug").Bool()
	if err != nil {
		return err
	}
	if debug {
		u.Debug = true
		u.SetAllLoggers(logging.DEBUG)
	}

	if u.Debug {
		stopProfilingFunc, err := startProfiling()
		if err != nil {
			return err
		}
		defer stopProfilingFunc() // to be executed as late as possible
	}

	helpTextDisplayed, err := handleHelpOption(req, root)
	if err != nil {
		return err
	}
	if helpTextDisplayed {
		return nil
	}

	res, err := callCommand(req, root)
	if err != nil {
		return err
	}

	err = outputResponse(res, root)
	if err != nil {
		return err
	}

	return nil
}

func createRequest(args []string) (cmds.Request, *cmds.Command, error) {
	req, root, cmd, path, err := cmdsCli.Parse(args, Root, commands.Root)

	// handle parse error (which means the commandline input was wrong,
	// e.g. incorrect number of args, or nonexistent subcommand)
	if err != nil {
		return nil, nil, handleParseError(req, root, cmd, path, err)
	}

	configPath, err := getConfigRoot(req)
	if err != nil {
		return nil, nil, err
	}

	conf, err := getConfig(configPath)
	if err != nil {
		return nil, nil, err
	}
	ctx := req.Context()
	ctx.ConfigRoot = configPath
	ctx.Config = conf

	// if no encoding was specified by user, default to plaintext encoding
	// (if command doesn't support plaintext, use JSON instead)
	if !req.Option("encoding").Found() {
		if req.Command().Marshallers != nil && req.Command().Marshallers[cmds.Text] != nil {
			req.SetOption("encoding", cmds.Text)
		} else {
			req.SetOption("encoding", cmds.JSON)
		}
	}

	return req, root, nil
}

func handleParseError(req cmds.Request, root *cmds.Command, cmd *cmds.Command, path []string, parseError error) error {
	var longHelp, shortHelp bool

	if req != nil {
		// help and h are defined in the root. We expect them to be bool.
		var err error
		longHelp, _, err = req.Option("help").Bool()
		if err != nil {
			return err
		}
		shortHelp, _, err = req.Option("h").Bool()
		if err != nil {
			return err
		}

		// override the error to avoid signaling other issues.
		parseError = errHelpRequested
	}

	// if the -help flag wasn't specified, show the error message
	// or if a path was returned (user specified a valid subcommand), show the error message
	// (this means there was an option or argument error)
	if path != nil && len(path) > 0 {
		if !longHelp && !shortHelp {
			fmt.Printf(errorFormat, parseError)
		}
	}

	if cmd == nil {
		root = commands.Root
	}

	// show the long help text if the -help flag was specified or we are at the root command
	// otherwise, show short help text
	helpFunc := cmdsCli.ShortHelp
	if longHelp || len(path) == 0 {
		helpFunc = cmdsCli.LongHelp
	}

	htErr := helpFunc("ipfs", root, path, os.Stdout)
	if htErr != nil {
		fmt.Println(htErr)
	}
	return parseError
}

func handleHelpOption(req cmds.Request, root *cmds.Command) (helpTextDisplayed bool, err error) {
	longHelp, _, err := req.Option("help").Bool()
	if err != nil {
		return false, err
	}
	shortHelp, _, err := req.Option("h").Bool()
	if err != nil {
		return false, err
	}
	if !longHelp && !shortHelp {
		return false, nil
	}
	helpFunc := cmdsCli.ShortHelp
	if longHelp || len(req.Path()) == 0 {
		helpFunc = cmdsCli.LongHelp
	}

	err = helpFunc("ipfs", root, req.Path(), os.Stdout)
	if err != nil {
		return false, err
	}
	return true, nil
}

func callCommand(req cmds.Request, root *cmds.Command) (cmds.Response, error) {
	var res cmds.Response

	if root == Root { // TODO explain what it means when root == Root
		res = root.Call(req)

	} else {
		local, found, err := req.Option("local").Bool()
		if err != nil {
			return nil, err
		}

		remote := !found || !local

		if remote && daemon.Locked(req.Context().ConfigRoot) {
			addr, err := ma.NewMultiaddr(req.Context().Config.Addresses.API)
			if err != nil {
				return nil, err
			}

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
			node, err := core.NewIpfsNode(req.Context().Config, false)
			if err != nil {
				return nil, err
			}
			defer node.Close()
			req.Context().Node = node

			res = root.Call(req)
		}
	}

	return res, nil
}

func outputResponse(res cmds.Response, root *cmds.Command) error {
	if res.Error() != nil {
		fmt.Printf(errorFormat, res.Error().Error())

		if res.Error().Code != cmds.ErrClient {
			return res.Error()
		}

		// if this is a client error, we try to display help text
		if res.Error().Code == cmds.ErrClient {
			err := cmdsCli.ShortHelp("ipfs", root, res.Request().Path(), os.Stdout)
			if err != nil {
				fmt.Println(err)
			}
		}

		emptyErr := errors.New("") // already displayed error text
		return emptyErr
	}

	out, err := res.Reader()
	if err != nil {
		return err
	}

	io.Copy(os.Stdout, out)
	return nil
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

func getConfig(path string) (*config.Config, error) {
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
