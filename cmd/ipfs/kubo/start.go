// cmd/ipfs/kubo implements the primary CLI binary for kubo
package kubo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/uuid"
	u "github.com/ipfs/boxo/util"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cli"
	cmdhttp "github.com/ipfs/go-ipfs-cmds/http"
	logging "github.com/ipfs/go-log"
	ipfs "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/client/rpc/auth"
	"github.com/ipfs/kubo/cmd/ipfs/util"
	oldcmds "github.com/ipfs/kubo/commands"
	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	corecmds "github.com/ipfs/kubo/core/commands"
	"github.com/ipfs/kubo/core/corehttp"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/ipfs/kubo/tracing"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	manet "github.com/multiformats/go-multiaddr/net"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// log is the command logger.
var (
	log    = logging.Logger("cmd/ipfs")
	tracer trace.Tracer
)

// declared as a var for testing purposes.
var dnsResolver = madns.DefaultResolver

const (
	EnvEnableProfiling = "IPFS_PROF"
	cpuProfile         = "ipfs.cpuprof"
	heapProfile        = "ipfs.memprof"
)

type PluginPreloader func(*loader.PluginLoader) error

func loadPlugins(repoPath string, preload PluginPreloader) (*loader.PluginLoader, error) {
	plugins, err := loader.NewPluginLoader(repoPath)
	if err != nil {
		return nil, fmt.Errorf("error loading plugins: %s", err)
	}

	if preload != nil {
		if err := preload(plugins); err != nil {
			return nil, fmt.Errorf("error loading plugins (preload): %s", err)
		}
	}

	if err := plugins.Initialize(); err != nil {
		return nil, fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return nil, fmt.Errorf("error initializing plugins: %s", err)
	}
	return plugins, nil
}

func printErr(err error) int {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	return 1
}

func newUUID(key string) logging.Metadata {
	ids := "#UUID-ERROR#"
	if id, err := uuid.NewRandom(); err == nil {
		ids = id.String()
	}
	return logging.Metadata{
		key: ids,
	}
}

func BuildDefaultEnv(ctx context.Context, req *cmds.Request) (cmds.Environment, error) {
	return BuildEnv(nil)(ctx, req)
}

// BuildEnv creates an environment to be used with the kubo CLI. Note: the plugin preloader should only call functions
// associated with preloaded plugins (i.e. Load).
func BuildEnv(pl PluginPreloader) func(ctx context.Context, req *cmds.Request) (cmds.Environment, error) {
	return func(ctx context.Context, req *cmds.Request) (cmds.Environment, error) {
		checkDebug(req)
		repoPath, err := getRepoPath(req)
		if err != nil {
			return nil, err
		}
		log.Debugf("config path is %s", repoPath)

		plugins, err := loadPlugins(repoPath, pl)
		if err != nil {
			return nil, err
		}

		// this sets up the function that will initialize the node
		// this is so that we can construct the node lazily.
		return &oldcmds.Context{
			ConfigRoot: repoPath,
			ReqLog:     &oldcmds.ReqLog{},
			Plugins:    plugins,
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

				return n, nil
			},
		}, nil
	}
}

// Start roadmap:
// - parse the commandline to get a cmdInvocation
// - if user requests help, print it and exit.
// - run the command invocation
// - output the response
// - if anything fails, print error, maybe with help.
func Start(buildEnv func(ctx context.Context, req *cmds.Request) (cmds.Environment, error)) (exitCode int) {
	ctx := logging.ContextWithLoggable(context.Background(), newUUID("session"))

	tp, err := tracing.NewTracerProvider(ctx)
	if err != nil {
		return printErr(err)
	}
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			exitCode = printErr(err)
		}
	}()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())
	tracer = tp.Tracer("Kubo-cli")

	stopFunc, err := profileIfEnabled()
	if err != nil {
		return printErr(err)
	}
	defer stopFunc() // to be executed as late as possible

	intrh, ctx := util.SetupInterruptHandler(ctx)
	defer intrh.Close()

	// Handle `ipfs version` or `ipfs help`
	if len(os.Args) > 1 {
		// Handle `ipfs --version'
		if os.Args[1] == "--version" {
			os.Args[1] = "version"
		}

		// Handle `ipfs help` and `ipfs help <sub-command>`
		if os.Args[1] == "help" {
			if len(os.Args) > 2 {
				os.Args = append(os.Args[:1], os.Args[2:]...)
				// Handle `ipfs help --help`
				// append `--help`,when the command is not `ipfs help --help`
				if os.Args[1] != "--help" {
					os.Args = append(os.Args, "--help")
				}
			} else {
				os.Args[1] = "--help"
			}
		}
	} else if insideGUI() { // if no args were passed, and we're in a GUI environment
		// launch the daemon instead of launching a ghost window
		os.Args = append(os.Args, "daemon", "--init")
	}

	// output depends on executable name passed in os.Args
	// so we need to make sure it's stable
	os.Args[0] = "ipfs"

	err = cli.Run(ctx, Root, os.Args, os.Stdin, os.Stdout, os.Stderr, buildEnv, makeExecutor)
	if err != nil {
		return 1
	}

	// everything went better than expected :)
	return 0
}

func insideGUI() bool {
	return util.InsideGUI()
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

func apiAddrOption(req *cmds.Request) (ma.Multiaddr, error) {
	apiAddrStr, apiSpecified := req.Options[corecmds.ApiOption].(string)
	if !apiSpecified {
		return nil, nil
	}
	return ma.NewMultiaddr(apiAddrStr)
}

// encodedAbsolutePathVersion is the version from which the absolute path header in
// multipart requests is %-encoded. Before this version, its sent raw.
var encodedAbsolutePathVersion = semver.MustParse("0.23.0-dev")

func makeExecutor(req *cmds.Request, env interface{}) (cmds.Executor, error) {
	exe := tracingWrappedExecutor{cmds.NewExecutor(req.Root)}
	cctx := env.(*oldcmds.Context)

	// Check if the command is disabled.
	if req.Command.NoLocal && req.Command.NoRemote {
		return nil, fmt.Errorf("command disabled: %v", req.Path)
	}

	// Can we just run this locally?
	if !req.Command.NoLocal {
		if doesNotUseRepo, ok := corecmds.GetDoesNotUseRepo(req.Command.Extra); doesNotUseRepo && ok {
			return exe, nil
		}
	}

	// Get the API option from the commandline.
	apiAddr, err := apiAddrOption(req)
	if err != nil {
		return nil, err
	}

	// Require that the command be run on the daemon when the API flag is
	// passed (unless we're trying to _run_ the daemon).
	daemonRequested := apiAddr != nil && req.Command != daemonCmd

	// Run this on the client if required.
	if req.Command.NoRemote {
		if daemonRequested {
			// User requested that the command be run on the daemon but we can't.
			// NOTE: We drop this check for the `ipfs daemon` command.
			return nil, errors.New("api flag specified but command cannot be run on the daemon")
		}
		return exe, nil
	}

	// Finally, look in the repo for an API file.
	if apiAddr == nil {
		var err error
		apiAddr, err = fsrepo.APIAddr(cctx.ConfigRoot)
		switch err {
		case nil, repo.ErrApiNotRunning:
		default:
			return nil, err
		}
	}

	// Still no api specified? Run it on the client or fail.
	if apiAddr == nil {
		if req.Command.NoLocal {
			return nil, fmt.Errorf("command must be run on the daemon: %v", req.Path)
		}
		return exe, nil
	}

	// Resolve the API addr.
	//
	// Do not replace apiAddr with the resolved addr so that the requested
	// hostname is kept for use in the request's HTTP header.
	_, err = resolveAddr(req.Context, apiAddr)
	if err != nil {
		return nil, err
	}
	network, host, err := manet.DialArgs(apiAddr)
	if err != nil {
		return nil, err
	}

	// Construct the executor.
	opts := []cmdhttp.ClientOpt{
		cmdhttp.ClientWithAPIPrefix(corehttp.APIPath),
	}

	// Fallback on a local executor if we (a) have a repo and (b) aren't
	// forcing a daemon.
	if !daemonRequested && fsrepo.IsInitialized(cctx.ConfigRoot) {
		opts = append(opts, cmdhttp.ClientWithFallback(exe))
	}

	var tpt http.RoundTripper
	switch network {
	case "tcp", "tcp4", "tcp6":
		tpt = http.DefaultTransport
		// RPC over HTTPS requires explicit schema in the address passed to cmdhttp.NewClient
		httpAddr := apiAddr.String()
		if !strings.HasPrefix(host, "http:") && !strings.HasPrefix(host, "https:") && (strings.Contains(httpAddr, "/https") || strings.Contains(httpAddr, "/tls/http")) {
			host = "https://" + host
		}
	case "unix":
		path := host
		host = "unix"
		tpt = &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		}
	default:
		return nil, fmt.Errorf("unsupported API address: %s", apiAddr)
	}

	apiAuth, specified := req.Options[corecmds.ApiAuthOption].(string)
	if specified {
		authorization := config.ConvertAuthSecret(apiAuth)
		tpt = auth.NewAuthorizedRoundTripper(authorization, tpt)
	}

	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(tpt),
	}
	opts = append(opts, cmdhttp.ClientWithHTTPClient(httpClient))

	// Fetch remove version, as some feature compatibility might change depending on it.
	remoteVersion, err := getRemoteVersion(tracingWrappedExecutor{cmdhttp.NewClient(host, opts...)})
	if err != nil {
		return nil, err
	}
	opts = append(opts, cmdhttp.ClientWithRawAbsPath(remoteVersion.LT(encodedAbsolutePathVersion)))

	return tracingWrappedExecutor{cmdhttp.NewClient(host, opts...)}, nil
}

type tracingWrappedExecutor struct {
	exec cmds.Executor
}

func (twe tracingWrappedExecutor) Execute(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	ctx, span := tracer.Start(req.Context, "cmds."+strings.Join(req.Path, "."), trace.WithAttributes(attribute.StringSlice("Arguments", req.Arguments)))
	defer span.End()
	req.Context = ctx

	err := twe.exec.Execute(req, re, env)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func getRepoPath(req *cmds.Request) (string, error) {
	repoOpt, found := req.Options[corecmds.RepoDirOption].(string)
	if found && repoOpt != "" {
		return repoOpt, nil
	}

	repoPath, err := fsrepo.BestKnownPath()
	if err != nil {
		return "", err
	}
	return repoPath, nil
}

// startProfiling begins CPU profiling and returns a `stop` function to be
// executed as late as possible. The stop function captures the memprofile.
func startProfiling() (func(), error) {
	// start CPU profiling as early as possible
	ofi, err := os.Create(cpuProfile)
	if err != nil {
		return nil, err
	}
	err = pprof.StartCPUProfile(ofi)
	if err != nil {
		ofi.Close()
		return nil, err
	}
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

func resolveAddr(ctx context.Context, addr ma.Multiaddr) (ma.Multiaddr, error) {
	ctx, cancelFunc := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFunc()

	addrs, err := dnsResolver.Resolve(ctx, addr)
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, errors.New("non-resolvable API endpoint")
	}

	return addrs[0], nil
}

type nopWriter struct {
	io.Writer
}

func (nw nopWriter) Close() error {
	return nil
}

func getRemoteVersion(exe cmds.Executor) (*semver.Version, error) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*30))
	defer cancel()

	req, err := cmds.NewRequest(ctx, []string{"version"}, nil, nil, nil, Root)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	re, err := cmds.NewWriterResponseEmitter(nopWriter{&buf}, req)
	if err != nil {
		return nil, err
	}

	err = exe.Execute(req, re, nil)
	if err != nil {
		return nil, err
	}

	var out ipfs.VersionInfo
	dec := json.NewDecoder(&buf)
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}

	return semver.New(out.Version)
}
