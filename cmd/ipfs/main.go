// cmd/ipfs implements the primary CLI binary for ipfs
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"sync"
	"syscall"
	"time"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	corecmds "github.com/ipfs/go-ipfs/core/commands"
	corehttp "github.com/ipfs/go-ipfs/core/corehttp"
	loader "github.com/ipfs/go-ipfs/plugin/loader"
	repo "github.com/ipfs/go-ipfs/repo"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds/cli"
	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds/http"
	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	"gx/ipfs/QmTyiSs9VgdVb4pnzdjtKhcfdTkHFEaNn6xnCbZq4DTFRt/go-ipfs-config"
	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	osh "gx/ipfs/QmXuBJ7DR6k3rmUEKtvVMhwjmXDuJgXXPUt4LQXKBMsU93/go-os-helper"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	loggables "gx/ipfs/QmZ4zF1mBrt8C2mSCM4ZYE4aAnv78f7GvrzufJC4G5tecK/go-libp2p-loggables"
)

// log is the command logger
var log = logging.Logger("cmd/ipfs")

var errRequestCanceled = errors.New("request canceled")

const (
	EnvEnableProfiling = "IPFS_PROF"
	cpuProfile         = "ipfs.cpuprof"
	heapProfile        = "ipfs.memprof"
)

// main roadmap:
// - parse the commandline to get a cmdInvocation
// - if user requests help, print it and exit.
// - run the command invocation
// - output the response
// - if anything fails, print error, maybe with help
func main() {
	os.Exit(mainRet())
}

func mainRet() int {
	rand.Seed(time.Now().UnixNano())
	ctx := logging.ContextWithLoggable(context.Background(), loggables.Uuid("session"))
	var err error

	// we'll call this local helper to output errors.
	// this is so we control how to print errors in one place.
	printErr := func(err error) {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}

	stopFunc, err := profileIfEnabled()
	if err != nil {
		printErr(err)
		return 1
	}
	defer stopFunc() // to be executed as late as possible

	intrh, ctx := setupInterruptHandler(ctx)
	defer intrh.Close()

	// Handle `ipfs help'
	if len(os.Args) == 2 {
		if os.Args[1] == "help" {
			os.Args[1] = "-h"
		} else if os.Args[1] == "--version" {
			os.Args[1] = "version"
		}
	}

	// output depends on executable name passed in os.Args
	// so we need to make sure it's stable
	os.Args[0] = "ipfs"

	buildEnv := func(ctx context.Context, req *cmds.Request) (cmds.Environment, error) {
		checkDebug(req)
		repoPath, err := getRepoPath(req)
		if err != nil {
			return nil, err
		}
		log.Debugf("config path is %s", repoPath)

		// this sets up the function that will initialize the node
		// this is so that we can construct the node lazily.
		return &oldcmds.Context{
			ConfigRoot: repoPath,
			LoadConfig: loadConfig,
			ReqLog:     &oldcmds.ReqLog{},
			ConstructNode: func() (n *core.IpfsNode, err error) {
				if req == nil {
					return nil, errors.New("constructing node without a request")
				}

				r, err := fsrepo.Open(repoPath)
				if err != nil { // repo is owned by the node
					return nil, err
				}

				// ok everything is good. set it on the invocation (for ownership)
				// and return it.
				n, err = core.NewNode(ctx, &core.BuildCfg{
					Repo: r,
				})
				if err != nil {
					return nil, err
				}

				n.SetLocal(true)
				return n, nil
			},
		}, nil
	}

	err = cli.Run(ctx, Root, os.Args, os.Stdin, os.Stdout, os.Stderr, buildEnv, makeExecutor)
	if err != nil {
		return 1
	}

	// everything went better than expected :)
	return 0
}

func checkDebug(req *cmds.Request) {
	// check if user wants to debug. option OR env var.
	debug, _ := req.Options["debug"].(bool)
	if debug || os.Getenv("IPFS_LOGGING") == "debug" {
		u.Debug = true
		logging.SetDebugLogging()
	}
	if u.GetenvBool("DEBUG") {
		u.Debug = true
	}
}

func makeExecutor(req *cmds.Request, env interface{}) (cmds.Executor, error) {
	details := commandDetails(req.Path)
	client, err := commandShouldRunOnDaemon(*details, req, env.(*oldcmds.Context))
	if err != nil {
		return nil, err
	}

	var exctr cmds.Executor
	if client != nil && !req.Command.External {
		exctr = client.(cmds.Executor)
	} else {
		cctx := env.(*oldcmds.Context)
		pluginpath := filepath.Join(cctx.ConfigRoot, "plugins")

		// check if repo is accessible before loading plugins
		ok, err := checkPermissions(cctx.ConfigRoot)
		if err != nil {
			return nil, err
		}
		if ok {
			if _, err := loader.LoadPlugins(pluginpath); err != nil {
				log.Error("error loading plugins: ", err)
			}
		}

		exctr = cmds.NewExecutor(req.Root)
	}

	return exctr, nil
}

func checkPermissions(path string) (bool, error) {
	_, err := os.Open(path)
	if os.IsNotExist(err) {
		// repo does not exist yet - don't load plugins, but also don't fail
		return false, nil
	}
	if os.IsPermission(err) {
		// repo is not accessible. error out.
		return false, fmt.Errorf("error opening repository at %s: permission denied", path)
	}

	return true, nil
}

// commandDetails returns a command's details for the command given by |path|.
func commandDetails(path []string) *cmdDetails {
	var details cmdDetails
	// find the last command in path that has a cmdDetailsMap entry
	for i := range path {
		if cmdDetails, found := cmdDetailsMap[strings.Join(path[:i+1], "/")]; found {
			details = cmdDetails
		}
	}
	return &details
}

// commandShouldRunOnDaemon determines, from command details, whether a
// command ought to be executed on an ipfs daemon.
//
// It returns a client if the command should be executed on a daemon and nil if
// it should be executed on a client. It returns an error if the command must
// NOT be executed on either.
func commandShouldRunOnDaemon(details cmdDetails, req *cmds.Request, cctx *oldcmds.Context) (http.Client, error) {
	path := req.Path
	// root command.
	if len(path) < 1 {
		return nil, nil
	}

	if details.cannotRunOnClient && details.cannotRunOnDaemon {
		return nil, fmt.Errorf("command disabled: %s", path[0])
	}

	if details.doesNotUseRepo && details.canRunOnClient() {
		return nil, nil
	}

	// at this point need to know whether api is running. we defer
	// to this point so that we don't check unnecessarily

	// did user specify an api to use for this command?
	apiAddrStr, _ := req.Options[corecmds.ApiOption].(string)

	client, err := getApiClient(cctx.ConfigRoot, apiAddrStr)
	if err == repo.ErrApiNotRunning {
		if apiAddrStr != "" && req.Command != daemonCmd {
			// if user SPECIFIED an api, and this cmd is not daemon
			// we MUST use it. so error out.
			return nil, err
		}

		// ok for api not to be running
	} else if err != nil { // some other api error
		return nil, err
	}

	if client != nil {
		if details.cannotRunOnDaemon {
			// check if daemon locked. legacy error text, for now.
			log.Debugf("Command cannot run on daemon. Checking if daemon is locked")
			if daemonLocked, _ := fsrepo.LockedByOtherProcess(cctx.ConfigRoot); daemonLocked {
				return nil, cmds.ClientError("ipfs daemon is running. please stop it to run this command")
			}
			return nil, nil
		}

		return client, nil
	}

	if details.cannotRunOnClient {
		return nil, cmds.ClientError("must run on the ipfs daemon")
	}

	return nil, nil
}

func getRepoPath(req *cmds.Request) (string, error) {
	repoOpt, found := req.Options["config"].(string)
	if found && repoOpt != "" {
		return repoOpt, nil
	}

	repoPath, err := fsrepo.BestKnownPath()
	if err != nil {
		return "", err
	}
	return repoPath, nil
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
	go func() {
		for range time.NewTicker(time.Second * 30).C {
			err := writeHeapProfileToFile()
			if err != nil {
				log.Error(err)
			}
		}
	}()

	stopProfiling := func() {
		pprof.StopCPUProfile()
		ofi.Close() // captured by the closure
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

// IntrHandler helps set up an interrupt handler that can
// be cleanly shut down through the io.Closer interface.
type IntrHandler struct {
	sig chan os.Signal
	wg  sync.WaitGroup
}

func NewIntrHandler() *IntrHandler {
	ih := &IntrHandler{}
	ih.sig = make(chan os.Signal, 1)
	return ih
}

func (ih *IntrHandler) Close() error {
	close(ih.sig)
	ih.wg.Wait()
	return nil
}

// Handle starts handling the given signals, and will call the handler
// callback function each time a signal is catched. The function is passed
// the number of times the handler has been triggered in total, as
// well as the handler itself, so that the handling logic can use the
// handler's wait group to ensure clean shutdown when Close() is called.
func (ih *IntrHandler) Handle(handler func(count int, ih *IntrHandler), sigs ...os.Signal) {
	signal.Notify(ih.sig, sigs...)
	ih.wg.Add(1)
	go func() {
		defer ih.wg.Done()
		count := 0
		for range ih.sig {
			count++
			handler(count, ih)
		}
		signal.Stop(ih.sig)
	}()
}

func setupInterruptHandler(ctx context.Context) (io.Closer, context.Context) {
	intrh := NewIntrHandler()
	ctx, cancelFunc := context.WithCancel(ctx)

	handlerFunc := func(count int, ih *IntrHandler) {
		switch count {
		case 1:
			fmt.Println() // Prevent un-terminated ^C character in terminal

			ih.wg.Add(1)
			go func() {
				defer ih.wg.Done()
				cancelFunc()
			}()

		default:
			fmt.Println("Received another interrupt before graceful shutdown, terminating...")
			os.Exit(-1)
		}
	}

	intrh.Handle(handlerFunc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	return intrh, ctx
}

func profileIfEnabled() (func(), error) {
	// FIXME this is a temporary hack so profiling of asynchronous operations
	// works as intended.
	if os.Getenv(EnvEnableProfiling) != "" {
		stopProfilingFunc, err := startProfiling() // TODO maybe change this to its own option... profiling makes it slower.
		if err != nil {
			return nil, err
		}
		return stopProfilingFunc, nil
	}
	return func() {}, nil
}

var apiFileErrorFmt string = `Failed to parse '%[1]s/api' file.
	error: %[2]s
If you're sure go-ipfs isn't running, you can just delete it.
`
var checkIPFSUnixFmt = "Otherwise check:\n\tps aux | grep ipfs"
var checkIPFSWinFmt = "Otherwise check:\n\ttasklist | findstr ipfs"

// getApiClient checks the repo, and the given options, checking for
// a running API service. if there is one, it returns a client.
// otherwise, it returns errApiNotRunning, or another error.
func getApiClient(repoPath, apiAddrStr string) (http.Client, error) {
	var apiErrorFmt string
	switch {
	case osh.IsUnix():
		apiErrorFmt = apiFileErrorFmt + checkIPFSUnixFmt
	case osh.IsWindows():
		apiErrorFmt = apiFileErrorFmt + checkIPFSWinFmt
	default:
		apiErrorFmt = apiFileErrorFmt
	}

	var addr ma.Multiaddr
	var err error
	if len(apiAddrStr) != 0 {
		addr, err = ma.NewMultiaddr(apiAddrStr)
		if err != nil {
			return nil, err
		}
		if len(addr.Protocols()) == 0 {
			return nil, fmt.Errorf("multiaddr doesn't provide any protocols")
		}
	} else {
		addr, err = fsrepo.APIAddr(repoPath)
		if err == repo.ErrApiNotRunning {
			return nil, err
		}

		if err != nil {
			return nil, fmt.Errorf(apiErrorFmt, repoPath, err.Error())
		}
	}
	if len(addr.Protocols()) == 0 {
		return nil, fmt.Errorf(apiErrorFmt, repoPath, "multiaddr doesn't provide any protocols")
	}
	return apiClientForAddr(addr)
}

func apiClientForAddr(addr ma.Multiaddr) (http.Client, error) {
	_, host, err := manet.DialArgs(addr)
	if err != nil {
		return nil, err
	}

	return http.NewClient(host, http.ClientWithAPIPrefix(corehttp.APIPath)), nil
}
