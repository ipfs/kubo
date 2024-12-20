package kubo

import (
	"context"
	"errors"
	_ "expvar"
	"fmt"
	"math"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"

	cmds "github.com/ipfs/go-ipfs-cmds"
	mprome "github.com/ipfs/go-metrics-prometheus"
	version "github.com/ipfs/kubo"
	utilmain "github.com/ipfs/kubo/cmd/ipfs/util"
	oldcmds "github.com/ipfs/kubo/commands"
	config "github.com/ipfs/kubo/config"
	cserial "github.com/ipfs/kubo/config/serialize"
	"github.com/ipfs/kubo/core"
	commands "github.com/ipfs/kubo/core/commands"
	"github.com/ipfs/kubo/core/coreapi"
	corehttp "github.com/ipfs/kubo/core/corehttp"
	options "github.com/ipfs/kubo/core/coreiface/options"
	corerepo "github.com/ipfs/kubo/core/corerepo"
	libp2p "github.com/ipfs/kubo/core/node/libp2p"
	nodeMount "github.com/ipfs/kubo/fuse/node"
	fsrepo "github.com/ipfs/kubo/repo/fsrepo"
	"github.com/ipfs/kubo/repo/fsrepo/migrations"
	"github.com/ipfs/kubo/repo/fsrepo/migrations/ipfsfetcher"
	goprocess "github.com/jbenet/goprocess"
	p2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	pnet "github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/protocol"
	p2phttp "github.com/libp2p/go-libp2p/p2p/http"
	sockets "github.com/libp2p/go-socket-activation"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	prometheus "github.com/prometheus/client_golang/prometheus"
	promauto "github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	adjustFDLimitKwd           = "manage-fdlimit"
	enableGCKwd                = "enable-gc"
	initOptionKwd              = "init"
	initConfigOptionKwd        = "init-config"
	initProfileOptionKwd       = "init-profile"
	ipfsMountKwd               = "mount-ipfs"
	ipnsMountKwd               = "mount-ipns"
	migrateKwd                 = "migrate"
	mountKwd                   = "mount"
	offlineKwd                 = "offline" // global option
	routingOptionKwd           = "routing"
	routingOptionSupernodeKwd  = "supernode"
	routingOptionDHTClientKwd  = "dhtclient"
	routingOptionDHTKwd        = "dht"
	routingOptionDHTServerKwd  = "dhtserver"
	routingOptionNoneKwd       = "none"
	routingOptionCustomKwd     = "custom"
	routingOptionDefaultKwd    = "default"
	routingOptionAutoKwd       = "auto"
	routingOptionAutoClientKwd = "autoclient"
	unencryptTransportKwd      = "disable-transport-encryption"
	unrestrictedAPIAccessKwd   = "unrestricted-api"
	enablePubSubKwd            = "enable-pubsub-experiment"
	enableIPNSPubSubKwd        = "enable-namesys-pubsub"
	enableMultiplexKwd         = "enable-mplex-experiment"
	agentVersionSuffix         = "agent-version-suffix"
	// apiAddrKwd    = "address-api"
	// swarmAddrKwd  = "address-swarm".
)

var daemonCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Run a network-connected IPFS node.",
		ShortDescription: `
'ipfs daemon' runs a persistent ipfs daemon that can serve commands
over the network. Most applications that use IPFS will do so by
communicating with a daemon over the HTTP API. While the daemon is
running, calls to 'ipfs' commands will be sent over the network to
the daemon.
`,
		LongDescription: `
The Kubo daemon will start listening on ports on the network, which are
documented in (and can be modified through) 'ipfs config Addresses'.
For example, to change the 'Gateway' port:

  ipfs config Addresses.Gateway /ip4/127.0.0.1/tcp/8082

The RPC API address can be changed the same way:

  ipfs config Addresses.API /ip4/127.0.0.1/tcp/5002

Make sure to restart the daemon after changing addresses.

By default, the gateway is only accessible locally. To expose it to
other computers in the network, use 0.0.0.0 as the ip address:

  ipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/8080

Be careful if you expose the RPC API. It is a security risk, as anyone could
control your node remotely. If you need to control the node remotely,
make sure to protect the port as you would other services or database
(firewall, authenticated proxy, etc), or at least set API.Authorizations.

If you do not want to open any ports for RPC, and only want to use
kubo CLI client, it is possible to expose the RPC over Unix socket:

  ipfs config Addresses.API /unix/var/run/kubo.socket

HTTP Headers

Kubo supports passing arbitrary headers to the RPC API and Gateway. You can
do this by setting headers on the API.HTTPHeaders and Gateway.HTTPHeaders
keys:

  ipfs config --json API.HTTPHeaders.X-Special-Header "[\"so special :)\"]"
  ipfs config --json Gateway.HTTPHeaders.X-Special-Header "[\"so special :)\"]"

Note that the value of the keys is an _array_ of strings. This is because
headers can have more than one value, and it is convenient to pass through
to other libraries.

CORS Headers (for RPC API)

You can setup CORS headers the same way:

  ipfs config --json API.HTTPHeaders.Access-Control-Allow-Origin "[\"example.com\"]"
  ipfs config --json API.HTTPHeaders.Access-Control-Allow-Methods "[\"PUT\", \"GET\", \"POST\"]"
  ipfs config --json API.HTTPHeaders.Access-Control-Allow-Credentials "[\"true\"]"

Shutdown

To shut down the daemon, send a SIGINT signal to it (e.g. by pressing 'Ctrl-C')
or send a SIGTERM signal to it (e.g. with 'kill'). It may take a while for the
daemon to shutdown gracefully, but it can be killed forcibly by sending a
second signal.

IPFS_PATH environment variable

Kubo uses a repository in the local file system. By default, the repo is
located at ~/.ipfs. To change the repo location, set the $IPFS_PATH
environment variable:

  export IPFS_PATH=/path/to/ipfsrepo

DEPRECATION NOTICE

Previously, Kubo used an environment variable as seen below:

  export API_ORIGIN="http://localhost:8888/"

This is deprecated. It is still honored in this version, but will be removed
in a future version, along with this notice. Please move to setting the HTTP
Headers.
`,
	},

	Options: []cmds.Option{
		cmds.BoolOption(initOptionKwd, "Initialize Kubo with default settings if not already initialized"),
		cmds.StringOption(initConfigOptionKwd, "Path to existing configuration file to be loaded during --init"),
		cmds.StringOption(initProfileOptionKwd, "Configuration profiles to apply for --init. See ipfs init --help for more"),
		cmds.StringOption(routingOptionKwd, "Overrides the routing option").WithDefault(routingOptionDefaultKwd),
		cmds.BoolOption(mountKwd, "Mounts IPFS to the filesystem using FUSE (experimental)"),
		cmds.StringOption(ipfsMountKwd, "Path to the mountpoint for IPFS (if using --mount). Defaults to config setting."),
		cmds.StringOption(ipnsMountKwd, "Path to the mountpoint for IPNS (if using --mount). Defaults to config setting."),
		cmds.BoolOption(unrestrictedAPIAccessKwd, "Allow RPC API access to unlisted hashes"),
		cmds.BoolOption(unencryptTransportKwd, "Disable transport encryption (for debugging protocols)"),
		cmds.BoolOption(enableGCKwd, "Enable automatic periodic repo garbage collection"),
		cmds.BoolOption(adjustFDLimitKwd, "Check and raise file descriptor limits if needed").WithDefault(true),
		cmds.BoolOption(migrateKwd, "If true, assume yes at the migrate prompt. If false, assume no."),
		cmds.BoolOption(enablePubSubKwd, "DEPRECATED"),
		cmds.BoolOption(enableIPNSPubSubKwd, "Enable IPNS over pubsub. Implicitly enables pubsub, overrides Ipns.UsePubsub config."),
		cmds.BoolOption(enableMultiplexKwd, "DEPRECATED"),
		cmds.StringOption(agentVersionSuffix, "Optional suffix to the AgentVersion presented by `ipfs id` and exposed via libp2p identify protocol."),

		// TODO: add way to override addresses. tricky part: updating the config if also --init.
		// cmds.StringOption(apiAddrKwd, "Address for the daemon rpc API (overrides config)"),
		// cmds.StringOption(swarmAddrKwd, "Address for the swarm socket (overrides config)"),
	},
	Subcommands: map[string]*cmds.Command{},
	NoRemote:    true,
	Extra:       commands.CreateCmdExtras(commands.SetDoesNotUseConfigAsInput(true)),
	Run:         daemonFunc,
}

// defaultMux tells mux to serve path using the default muxer. This is
// mostly useful to hook up things that register in the default muxer,
// and don't provide a convenient http.Handler entry point, such as
// expvar and http/pprof.
func defaultMux(path string) corehttp.ServeOption {
	return func(node *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.Handle(path, http.DefaultServeMux)
		return mux, nil
	}
}

func daemonFunc(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) (_err error) {
	// Inject metrics before we do anything
	err := mprome.Inject()
	if err != nil {
		log.Errorf("Injecting prometheus handler for metrics failed with message: %s\n", err.Error())
	}

	// let the user know we're going.
	fmt.Printf("Initializing daemon...\n")

	defer func() {
		if _err != nil {
			// Print an extra line before any errors. This could go
			// in the commands lib but doesn't really make sense for
			// all commands.
			fmt.Println()
		}
	}()

	// print the ipfs version
	printVersion()

	managefd, _ := req.Options[adjustFDLimitKwd].(bool)
	if managefd {
		if _, _, err := utilmain.ManageFdLimit(); err != nil {
			log.Errorf("setting file descriptor limit: %s", err)
		}
	}

	cctx := env.(*oldcmds.Context)

	// check transport encryption flag.
	unencrypted, _ := req.Options[unencryptTransportKwd].(bool)
	if unencrypted {
		log.Warnf(`Running with --%s: All connections are UNENCRYPTED.
		You will not be able to connect to regular encrypted networks.`, unencryptTransportKwd)
	}

	// first, whether user has provided the initialization flag. we may be
	// running in an uninitialized state.
	initialize, _ := req.Options[initOptionKwd].(bool)
	if initialize && !fsrepo.IsInitialized(cctx.ConfigRoot) {
		cfgLocation, _ := req.Options[initConfigOptionKwd].(string)
		profiles, _ := req.Options[initProfileOptionKwd].(string)
		var conf *config.Config

		if cfgLocation != "" {
			if conf, err = cserial.Load(cfgLocation); err != nil {
				return err
			}
		}

		if conf == nil {
			identity, err := config.CreateIdentity(os.Stdout, []options.KeyGenerateOption{
				options.Key.Type(algorithmDefault),
			})
			if err != nil {
				return err
			}
			conf, err = config.InitWithIdentity(identity)
			if err != nil {
				return err
			}
		}

		if err = doInit(os.Stdout, cctx.ConfigRoot, false, profiles, conf); err != nil {
			return err
		}
	}

	var cacheMigrations, pinMigrations bool
	var fetcher migrations.Fetcher

	// acquire the repo lock _before_ constructing a node. we need to make
	// sure we are permitted to access the resources (datastore, etc.)
	repo, err := fsrepo.Open(cctx.ConfigRoot)
	switch err {
	default:
		return err
	case fsrepo.ErrNeedMigration:
		domigrate, found := req.Options[migrateKwd].(bool)
		fmt.Println("Found outdated fs-repo, migrations need to be run.")

		if !found {
			domigrate = YesNoPrompt("Run migrations now? [y/N]")
		}

		if !domigrate {
			fmt.Println("Not running migrations of fs-repo now.")
			fmt.Println("Please get fs-repo-migrations from https://dist.ipfs.tech")
			return errors.New("fs-repo requires migration")
		}

		// Read Migration section of IPFS config
		configFileOpt, _ := req.Options[commands.ConfigFileOption].(string)
		migrationCfg, err := migrations.ReadMigrationConfig(cctx.ConfigRoot, configFileOpt)
		if err != nil {
			return err
		}

		// Define function to create IPFS fetcher.  Do not supply an
		// already-constructed IPFS fetcher, because this may be expensive and
		// not needed according to migration config. Instead, supply a function
		// to construct the particular IPFS fetcher implementation used here,
		// which is called only if an IPFS fetcher is needed.
		newIpfsFetcher := func(distPath string) migrations.Fetcher {
			return ipfsfetcher.NewIpfsFetcher(distPath, 0, &cctx.ConfigRoot, configFileOpt)
		}

		// Fetch migrations from current distribution, or location from environ
		fetchDistPath := migrations.GetDistPathEnv(migrations.CurrentIpfsDist)

		// Create fetchers according to migrationCfg.DownloadSources
		fetcher, err = migrations.GetMigrationFetcher(migrationCfg.DownloadSources, fetchDistPath, newIpfsFetcher)
		if err != nil {
			return err
		}
		defer fetcher.Close()

		if migrationCfg.Keep == "cache" {
			cacheMigrations = true
		} else if migrationCfg.Keep == "pin" {
			pinMigrations = true
		}

		if cacheMigrations || pinMigrations {
			// Create temp directory to store downloaded migration archives
			migrations.DownloadDirectory, err = os.MkdirTemp("", "migrations")
			if err != nil {
				return err
			}
			// Defer cleanup of download directory so that it gets cleaned up
			// if daemon returns early due to error
			defer func() {
				if migrations.DownloadDirectory != "" {
					os.RemoveAll(migrations.DownloadDirectory)
				}
			}()
		}

		err = migrations.RunMigration(cctx.Context(), fetcher, fsrepo.RepoVersion, "", false)
		if err != nil {
			fmt.Println("The migrations of fs-repo failed:")
			fmt.Printf("  %s\n", err)
			fmt.Println("If you think this is a bug, please file an issue and include this whole log output.")
			fmt.Println("  https://github.com/ipfs/fs-repo-migrations")
			return err
		}

		repo, err = fsrepo.Open(cctx.ConfigRoot)
		if err != nil {
			return err
		}
	case nil:
		break
	}

	// The node will also close the repo but there are many places we could
	// fail before we get to that. It can't hurt to close it twice.
	defer repo.Close()

	offline, _ := req.Options[offlineKwd].(bool)
	ipnsps, ipnsPsSet := req.Options[enableIPNSPubSubKwd].(bool)
	pubsub, psSet := req.Options[enablePubSubKwd].(bool)

	if _, hasMplex := req.Options[enableMultiplexKwd]; hasMplex {
		log.Errorf("The mplex multiplexer has been enabled by default and the experimental %s flag has been removed.")
		log.Errorf("To disable this multiplexer, please configure `Swarm.Transports.Multiplexers'.")
	}

	cfg, err := repo.Config()
	if err != nil {
		return err
	}

	fmt.Printf("PeerID: %s\n", cfg.Identity.PeerID)

	if !psSet {
		pubsub = cfg.Pubsub.Enabled.WithDefault(false)
	}
	if !ipnsPsSet {
		ipnsps = cfg.Ipns.UsePubsub.WithDefault(false)
	}

	// Start assembling node config
	ncfg := &core.BuildCfg{
		Repo:                        repo,
		Permanent:                   true, // It is temporary way to signify that node is permanent
		Online:                      !offline,
		DisableEncryptedConnections: unencrypted,
		ExtraOpts: map[string]bool{
			"pubsub": pubsub,
			"ipnsps": ipnsps,
		},
		// TODO(Kubuxu): refactor Online vs Offline by adding Permanent vs Ephemeral
	}

	routingOption, _ := req.Options[routingOptionKwd].(string)
	if routingOption == routingOptionDefaultKwd {
		routingOption = cfg.Routing.Type.WithDefault(routingOptionAutoKwd)
		if routingOption == "" {
			routingOption = routingOptionAutoKwd
		}
	}

	if key, _ := repo.SwarmKey(); key != nil || pnet.ForcePrivateNetwork {
		// Private setups can't leverage peers returned by default IPNIs (Routing.Type=auto)
		// To avoid breaking existing setups, switch them to DHT-only.
		if routingOption == routingOptionAutoKwd {
			log.Error("Private networking (swarm.key / LIBP2P_FORCE_PNET) does not work with public HTTP IPNIs enabled by Routing.Type=auto. Kubo will use Routing.Type=dht instead. Update config to remove this message.")
			routingOption = routingOptionDHTKwd
		}

		// Private setups should not use public AutoTLS infrastructure
		// as it will leak their existence and PeerID identity to CA
		// and they will show up at https://crt.sh/?q=libp2p.direct
		// Below ensures we hard fail if someone tries to enable both
		if cfg.AutoTLS.Enabled.WithDefault(config.DefaultAutoTLSEnabled) {
			return errors.New("private networking (swarm.key / LIBP2P_FORCE_PNET) does not work with AutoTLS.Enabled=true, update config to remove this message")
		}
	}

	switch routingOption {
	case routingOptionSupernodeKwd:
		return errors.New("supernode routing was never fully implemented and has been removed")
	case routingOptionDefaultKwd, routingOptionAutoKwd:
		ncfg.Routing = libp2p.ConstructDefaultRouting(cfg, libp2p.DHTOption)
	case routingOptionAutoClientKwd:
		ncfg.Routing = libp2p.ConstructDefaultRouting(cfg, libp2p.DHTClientOption)
	case routingOptionDHTClientKwd:
		ncfg.Routing = libp2p.DHTClientOption
	case routingOptionDHTKwd:
		ncfg.Routing = libp2p.DHTOption
	case routingOptionDHTServerKwd:
		ncfg.Routing = libp2p.DHTServerOption
	case routingOptionNoneKwd:
		ncfg.Routing = libp2p.NilRouterOption
	case routingOptionCustomKwd:
		if cfg.Routing.AcceleratedDHTClient.WithDefault(config.DefaultAcceleratedDHTClient) {
			return errors.New("Routing.AcceleratedDHTClient option is set even tho Routing.Type is custom, using custom .AcceleratedDHTClient needs to be set on DHT routers individually")
		}
		ncfg.Routing = libp2p.ConstructDelegatedRouting(
			cfg.Routing.Routers,
			cfg.Routing.Methods,
			cfg.Identity.PeerID,
			cfg.Addresses,
			cfg.Identity.PrivKey,
		)
	default:
		return fmt.Errorf("unrecognized routing option: %s", routingOption)
	}

	// Set optional agent version suffix
	versionSuffixFromCli, _ := req.Options[agentVersionSuffix].(string)
	versionSuffix := cfg.Version.AgentSuffix.WithDefault(versionSuffixFromCli)
	if versionSuffix != "" {
		version.SetUserAgentSuffix(versionSuffix)
	}

	node, err := core.NewNode(req.Context, ncfg)
	if err != nil {
		return err
	}
	node.IsDaemon = true

	if node.PNetFingerprint != nil {
		fmt.Println("Swarm is limited to private network of peers with the swarm key")
		fmt.Printf("Swarm key fingerprint: %x\n", node.PNetFingerprint)
	}

	if (pnet.ForcePrivateNetwork || node.PNetFingerprint != nil) && (routingOption == routingOptionAutoKwd || routingOption == routingOptionAutoClientKwd) {
		// This should never happen, but better safe than sorry
		log.Fatal("Private network does not work with Routing.Type=auto. Update your config to Routing.Type=dht (or none, and do manual peering)")
	}

	printLibp2pPorts(node)

	if node.PrivateKey.Type() == p2pcrypto.RSA {
		fmt.Print(`
Warning: You are using an RSA Peer ID, which was replaced by Ed25519
as the default recommended in Kubo since September 2020. Signing with
RSA Peer IDs is more CPU-intensive than with other key types.
It is recommended that you change your public key type to ed25519
by using the following command:

  ipfs key rotate -o rsa-key-backup -t ed25519

After changing your key type, restart your node for the changes to
take effect.

`)
	}

	defer func() {
		// We wait for the node to close first, as the node has children
		// that it will wait for before closing, such as the API server.
		node.Close()

		select {
		case <-req.Context.Done():
			log.Info("Gracefully shut down daemon")
		default:
		}
	}()

	cctx.ConstructNode = func() (*core.IpfsNode, error) {
		return node, nil
	}

	// Start "core" plugins. We want to do this *before* starting the HTTP
	// API as the user may be relying on these plugins.
	err = cctx.Plugins.Start(node)
	if err != nil {
		return err
	}
	select {
	case <-node.Process.Closing():
	default:
		node.Process.AddChild(goprocess.WithTeardown(cctx.Plugins.Close))
	}

	// construct api endpoint - every time
	apiErrc, err := serveHTTPApi(req, cctx)
	if err != nil {
		return err
	}

	// construct fuse mountpoints - if the user provided the --mount flag
	mount, _ := req.Options[mountKwd].(bool)
	if mount && offline {
		return cmds.Errorf(cmds.ErrClient, "mount is not currently supported in offline mode")
	}
	if mount {
		if err := mountFuse(req, cctx); err != nil {
			return err
		}
	}

	// repo blockstore GC - if --enable-gc flag is present
	gcErrc, err := maybeRunGC(req, node)
	if err != nil {
		return err
	}

	// Add any files downloaded by migration.
	if cacheMigrations || pinMigrations {
		err = addMigrations(cctx.Context(), node, fetcher, pinMigrations)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not add migration to IPFS:", err)
		}
		// Remove download directory so that it does not remain for lifetime of
		// daemon or get left behind if daemon has a hard exit
		os.RemoveAll(migrations.DownloadDirectory)
		migrations.DownloadDirectory = ""
	}
	if fetcher != nil {
		// If there is an error closing the IpfsFetcher, then print error, but
		// do not fail because of it.
		err = fetcher.Close()
		if err != nil {
			log.Errorf("error closing IPFS fetcher: %s", err)
		}
	}

	// construct http gateway
	gwErrc, err := serveHTTPGateway(req, cctx)
	if err != nil {
		return err
	}

	// add trustless gateway over libp2p
	p2pGwErrc, err := serveTrustlessGatewayOverLibp2p(cctx)
	if err != nil {
		return err
	}

	// Add ipfs version info to prometheus metrics
	ipfsInfoMetric := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ipfs_info",
		Help: "Kubo IPFS version information.",
	}, []string{"version", "commit"})

	// Setting to 1 lets us multiply it with other stats to add the version labels
	ipfsInfoMetric.With(prometheus.Labels{
		"version": version.CurrentVersionNumber,
		"commit":  version.CurrentCommit,
	}).Set(1)

	// TODO(9285): make metrics more configurable
	// initialize metrics collector
	prometheus.MustRegister(&corehttp.IpfsNodeCollector{Node: node})

	// start MFS pinning thread
	startPinMFS(cctx, daemonConfigPollInterval, &ipfsPinMFSNode{node})

	// The daemon is *finally* ready.
	fmt.Printf("Daemon is ready\n")
	notifyReady()

	// Give the user some immediate feedback when they hit C-c
	go func() {
		<-req.Context.Done()
		notifyStopping()
		fmt.Println("Received interrupt signal, shutting down...")
		fmt.Println("(Hit ctrl-c again to force-shutdown the daemon.)")
	}()

	if !offline {
		// Warn users who were victims of 'lowprofile' footgun (https://github.com/ipfs/kubo/pull/10524)
		if cfg.Experimental.StrategicProviding {
			fmt.Print(`
⚠️ Reprovide system is disabled due to 'Experimental.StrategicProviding=true'
⚠️ Local CIDs will not be announced to Amino DHT, making them impossible to retrieve without manual peering
⚠️ If this is not intentional, call 'ipfs config profile apply announce-on'

`)
		} else if cfg.Reprovider.Interval.WithDefault(config.DefaultReproviderInterval) == 0 {
			fmt.Print(`
⚠️ Reprovider system is disabled due to 'Reprovider.Interval=0'
⚠️ Local CIDs will not be announced to Amino DHT, making them impossible to retrieve without manual peering
⚠️ If this is not intentional, call 'ipfs config profile apply announce-on', or set 'Reprovider.Interval=22h'

`)
		}

		// Give the user heads up if daemon running in online mode has no peers after 1 minute
		time.AfterFunc(1*time.Minute, func() {
			cfg, err := cctx.GetConfig()
			if err != nil {
				log.Errorf("failed to access config: %s", err)
				return
			}
			if len(cfg.Bootstrap) == 0 && len(cfg.Peering.Peers) == 0 {
				// Skip peer check if Bootstrap and Peering lists are empty
				// (means user disabled them on purpose)
				log.Warn("skipping bootstrap: empty Bootstrap and Peering lists")
				return
			}
			ipfs, err := coreapi.NewCoreAPI(node)
			if err != nil {
				log.Errorf("failed to access CoreAPI: %v", err)
				return
			}
			peers, err := ipfs.Swarm().Peers(cctx.Context())
			if err != nil {
				log.Errorf("failed to read swarm peers: %v", err)
				return
			}
			if len(peers) == 0 {
				log.Error("failed to bootstrap (no peers found): consider updating Bootstrap or Peering section of your config")
			} else {
				// After 1 minute we should have enough peers
				// to run informed version check
				startVersionChecker(
					cctx.Context(),
					node,
					cfg.Version.SwarmCheckEnabled.WithDefault(true),
					cfg.Version.SwarmCheckPercentThreshold.WithDefault(config.DefaultSwarmCheckPercentThreshold),
				)
			}
		})
	}

	// Hard deprecation notice if someone still uses IPFS_REUSEPORT
	if flag := os.Getenv("IPFS_REUSEPORT"); flag != "" {
		log.Fatal("Support for IPFS_REUSEPORT was removed. Use LIBP2P_TCP_REUSEPORT instead.")
	}

	// collect long-running errors and block for shutdown
	// TODO(cryptix): our fuse currently doesn't follow this pattern for graceful shutdown
	var errs error
	for err := range merge(apiErrc, gwErrc, gcErrc, p2pGwErrc) {
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

// serveHTTPApi collects options, creates listener, prints status message and starts serving requests.
func serveHTTPApi(req *cmds.Request, cctx *oldcmds.Context) (<-chan error, error) {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: GetConfig() failed: %s", err)
	}

	listeners, err := sockets.TakeListeners("io.ipfs.api")
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: socket activation failed: %s", err)
	}

	apiAddrs := make([]string, 0, 2)
	apiAddr, _ := req.Options[commands.ApiOption].(string)
	if apiAddr == "" {
		apiAddrs = cfg.Addresses.API
	} else {
		apiAddrs = append(apiAddrs, apiAddr)
	}

	listenerAddrs := make(map[string]bool, len(listeners))
	for _, listener := range listeners {
		listenerAddrs[string(listener.Multiaddr().Bytes())] = true
	}

	for _, addr := range apiAddrs {
		apiMaddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("serveHTTPApi: invalid API address: %q (err: %s)", addr, err)
		}
		if listenerAddrs[string(apiMaddr.Bytes())] {
			continue
		}

		apiLis, err := manet.Listen(apiMaddr)
		if err != nil {
			return nil, fmt.Errorf("serveHTTPApi: manet.Listen(%s) failed: %s", apiMaddr, err)
		}

		listenerAddrs[string(apiMaddr.Bytes())] = true
		listeners = append(listeners, apiLis)
	}

	if len(cfg.API.Authorizations) > 0 && len(listeners) > 0 {
		fmt.Printf("RPC API access is limited by the rules defined in API.Authorizations\n")
	}

	for _, listener := range listeners {
		// we might have listened to /tcp/0 - let's see what we are listing on
		fmt.Printf("RPC API server listening on %s\n", listener.Multiaddr())
		// Browsers require TCP with explicit host.
		switch listener.Addr().Network() {
		case "tcp", "tcp4", "tcp6":
			rpc := listener.Addr().String()
			// replace catch-all with explicit localhost URL that works in browsers
			// https://github.com/ipfs/kubo/issues/10515
			if strings.Contains(rpc, "0.0.0.0:") {
				rpc = strings.Replace(rpc, "0.0.0.0:", "127.0.0.1:", 1)
			} else if strings.Contains(rpc, "[::]:") {
				rpc = strings.Replace(rpc, "[::]:", "[::1]:", 1)
			}
			fmt.Printf("WebUI: http://%s/webui\n", rpc)
		}
	}

	// by default, we don't let you load arbitrary ipfs objects through the api,
	// because this would open up the api to scripting vulnerabilities.
	// only the webui objects are allowed.
	// if you know what you're doing, go ahead and pass --unrestricted-api.
	unrestricted, _ := req.Options[unrestrictedAPIAccessKwd].(bool)
	gatewayOpt := corehttp.GatewayOption(corehttp.WebUIPaths...)
	if unrestricted {
		gatewayOpt = corehttp.GatewayOption("/ipfs", "/ipns")
	}

	opts := []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("api"),
		corehttp.MetricsOpenCensusCollectionOption(),
		corehttp.MetricsOpenCensusDefaultPrometheusRegistry(),
		corehttp.CheckVersionOption(),
		corehttp.CommandsOption(*cctx),
		corehttp.WebUIOption,
		gatewayOpt,
		corehttp.VersionOption(),
		defaultMux("/debug/vars"),
		defaultMux("/debug/pprof/"),
		defaultMux("/debug/stack"),
		corehttp.MutexFractionOption("/debug/pprof-mutex/"),
		corehttp.BlockProfileRateOption("/debug/pprof-block/"),
		corehttp.MetricsScrapingOption("/debug/metrics/prometheus"),
		corehttp.LogOption(),
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	node, err := cctx.ConstructNode()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: ConstructNode() failed: %s", err)
	}

	if len(listeners) > 0 {
		// Only add an api file if the API is running.
		if err := node.Repo.SetAPIAddr(rewriteMaddrToUseLocalhostIfItsAny(listeners[0].Multiaddr())); err != nil {
			return nil, fmt.Errorf("serveHTTPApi: SetAPIAddr() failed: %w", err)
		}
	}

	errc := make(chan error)
	var wg sync.WaitGroup
	for _, apiLis := range listeners {
		wg.Add(1)
		go func(lis manet.Listener) {
			defer wg.Done()
			errc <- corehttp.Serve(node, manet.NetListener(lis), opts...)
		}(apiLis)
	}

	go func() {
		wg.Wait()
		close(errc)
	}()

	return errc, nil
}

func rewriteMaddrToUseLocalhostIfItsAny(maddr ma.Multiaddr) ma.Multiaddr {
	first, rest := ma.SplitFirst(maddr)

	switch {
	case first.Equal(manet.IP4Unspecified):
		return manet.IP4Loopback.Encapsulate(rest)
	case first.Equal(manet.IP6Unspecified):
		return manet.IP6Loopback.Encapsulate(rest)
	default:
		return maddr // not ip
	}
}

// printLibp2pPorts prints which ports are opened to facilitate swarm connectivity.
func printLibp2pPorts(node *core.IpfsNode) {
	if !node.IsOnline {
		fmt.Println("Swarm not listening, running in offline mode.")
		return
	}

	ifaceAddrs, err := node.PeerHost.Network().InterfaceListenAddresses()
	if err != nil {
		log.Errorf("failed to read listening addresses: %s", err)
	}

	// Multiple libp2p transports can use same port.
	// Deduplicate all listeners and collect unique IP:port (udp|tcp) combinations
	// which is useful information for operator deploying Kubo in TCP/IP infra.
	addrMap := make(map[string]map[string]struct{})
	re := regexp.MustCompile(`^/(?:ip[46]|dns(?:[46])?)/([^/]+)/(tcp|udp)/(\d+)(/.*)?$`)
	for _, addr := range ifaceAddrs {
		matches := re.FindStringSubmatch(addr.String())
		if matches != nil {
			hostname := matches[1]
			protocol := strings.ToUpper(matches[2])
			port := matches[3]
			var host string
			if matches[0][:4] == "/ip6" {
				host = fmt.Sprintf("[%s]:%s", hostname, port)
			} else {
				host = fmt.Sprintf("%s:%s", hostname, port)
			}
			if _, ok := addrMap[host]; !ok {
				addrMap[host] = make(map[string]struct{})
			}
			addrMap[host][protocol] = struct{}{}
		}
	}

	// Produce a sorted host:port list
	hosts := make([]string, 0, len(addrMap))
	for host := range addrMap {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	// Print listeners
	for _, host := range hosts {
		protocolsSet := addrMap[host]
		protocols := make([]string, 0, len(protocolsSet))
		for protocol := range protocolsSet {
			protocols = append(protocols, protocol)
		}
		sort.Strings(protocols)
		fmt.Printf("Swarm listening on %s (%s)\n", host, strings.Join(protocols, "+"))
	}
	fmt.Printf("Run 'ipfs id' to inspect announced and discovered multiaddrs of this node.\n")
}

// serveHTTPGateway collects options, creates listener, prints status message and starts serving requests.
func serveHTTPGateway(req *cmds.Request, cctx *oldcmds.Context) (<-chan error, error) {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPGateway: GetConfig() failed: %s", err)
	}

	listeners, err := sockets.TakeListeners("io.ipfs.gateway")
	if err != nil {
		return nil, fmt.Errorf("serveHTTPGateway: socket activation failed: %s", err)
	}

	listenerAddrs := make(map[string]bool, len(listeners))
	for _, listener := range listeners {
		listenerAddrs[string(listener.Multiaddr().Bytes())] = true
	}

	gatewayAddrs := cfg.Addresses.Gateway
	for _, addr := range gatewayAddrs {
		gatewayMaddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("serveHTTPGateway: invalid gateway address: %q (err: %s)", addr, err)
		}

		if listenerAddrs[string(gatewayMaddr.Bytes())] {
			continue
		}

		gwLis, err := manet.Listen(gatewayMaddr)
		if err != nil {
			return nil, fmt.Errorf("serveHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err)
		}
		listenerAddrs[string(gatewayMaddr.Bytes())] = true
		listeners = append(listeners, gwLis)
	}

	// we might have listened to /tcp/0 - let's see what we are listing on
	for _, listener := range listeners {
		fmt.Printf("Gateway server listening on %s\n", listener.Multiaddr())
	}

	if cfg.Gateway.ExposeRoutingAPI.WithDefault(config.DefaultExposeRoutingAPI) {
		for _, listener := range listeners {
			fmt.Printf("Routing V1 API exposed at http://%s/routing/v1\n", listener.Addr())
		}
	}

	cmdctx := *cctx
	cmdctx.Gateway = true

	opts := []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("gateway"),
		corehttp.HostnameOption(),
		corehttp.GatewayOption("/ipfs", "/ipns"),
		corehttp.VersionOption(),
		corehttp.CheckVersionOption(),
	}

	if cfg.Experimental.P2pHttpProxy {
		opts = append(opts, corehttp.P2PProxyOption())
	}

	if cfg.Gateway.ExposeRoutingAPI.WithDefault(config.DefaultExposeRoutingAPI) {
		opts = append(opts, corehttp.RoutingOption())
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	node, err := cctx.ConstructNode()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPGateway: ConstructNode() failed: %s", err)
	}

	if len(listeners) > 0 {
		addr, err := manet.ToNetAddr(rewriteMaddrToUseLocalhostIfItsAny(listeners[0].Multiaddr()))
		if err != nil {
			return nil, fmt.Errorf("serveHTTPGateway: manet.ToIP() failed: %w", err)
		}
		if err := node.Repo.SetGatewayAddr(addr); err != nil {
			return nil, fmt.Errorf("serveHTTPGateway: SetGatewayAddr() failed: %w", err)
		}
	}

	errc := make(chan error)
	var wg sync.WaitGroup
	for _, lis := range listeners {
		wg.Add(1)
		go func(lis manet.Listener) {
			defer wg.Done()
			errc <- corehttp.Serve(node, manet.NetListener(lis), opts...)
		}(lis)
	}

	go func() {
		wg.Wait()
		close(errc)
	}()

	return errc, nil
}

const gatewayProtocolID protocol.ID = "/ipfs/gateway" // FIXME: specify https://github.com/ipfs/specs/issues/433

func serveTrustlessGatewayOverLibp2p(cctx *oldcmds.Context) (<-chan error, error) {
	node, err := cctx.ConstructNode()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPGatewayOverLibp2p: ConstructNode() failed: %s", err)
	}
	cfg, err := node.Repo.Config()
	if err != nil {
		return nil, fmt.Errorf("could not read config: %w", err)
	}

	if !cfg.Experimental.GatewayOverLibp2p {
		errCh := make(chan error)
		close(errCh)
		return errCh, nil
	}

	opts := []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("libp2p-gateway"),
		corehttp.Libp2pGatewayOption(),
		corehttp.VersionOption(),
	}

	handler, err := corehttp.MakeHandler(node, nil, opts...)
	if err != nil {
		return nil, err
	}

	h := p2phttp.Host{
		StreamHost: node.PeerHost,
	}

	h.WellKnownHandler.AddProtocolMeta(gatewayProtocolID, p2phttp.ProtocolMeta{Path: "/"})
	h.ServeMux = http.NewServeMux()
	h.ServeMux.Handle("/", handler)

	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		errc <- h.Serve()
	}()

	go func() {
		<-node.Process.Closing()
		h.Close()
	}()

	return errc, nil
}

// collects options and opens the fuse mountpoint.
func mountFuse(req *cmds.Request, cctx *oldcmds.Context) error {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return fmt.Errorf("mountFuse: GetConfig() failed: %s", err)
	}

	fsdir, found := req.Options[ipfsMountKwd].(string)
	if !found {
		fsdir = cfg.Mounts.IPFS
	}

	nsdir, found := req.Options[ipnsMountKwd].(string)
	if !found {
		nsdir = cfg.Mounts.IPNS
	}

	node, err := cctx.ConstructNode()
	if err != nil {
		return fmt.Errorf("mountFuse: ConstructNode() failed: %s", err)
	}

	err = nodeMount.Mount(node, fsdir, nsdir)
	if err != nil {
		return err
	}
	fmt.Printf("IPFS mounted at: %s\n", fsdir)
	fmt.Printf("IPNS mounted at: %s\n", nsdir)
	return nil
}

func maybeRunGC(req *cmds.Request, node *core.IpfsNode) (<-chan error, error) {
	enableGC, _ := req.Options[enableGCKwd].(bool)
	if !enableGC {
		return nil, nil
	}

	errc := make(chan error)
	go func() {
		errc <- corerepo.PeriodicGC(req.Context, node)
		close(errc)
	}()
	return errc, nil
}

// merge does fan-in of multiple read-only error channels
// taken from http://blog.golang.org/pipelines
func merge(cs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error)

	// Start an output goroutine for each input channel in cs.  output
	// copies values from c to out until c is closed, then calls wg.Done.
	output := func(c <-chan error) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	for _, c := range cs {
		if c != nil {
			wg.Add(1)
			go output(c)
		}
	}

	// Start a goroutine to close out once all the output goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func YesNoPrompt(prompt string) bool {
	var s string
	for i := 0; i < 3; i++ {
		fmt.Printf("%s ", prompt)
		_, err := fmt.Scanf("%s", &s)
		if err != nil {
			fmt.Printf("Invalid input: %v. Please try again.\n", err)
			continue
		}
		switch s {
		case "y", "Y":
			return true
		case "n", "N":
			return false
		case "":
			return false
		}
		fmt.Println("Please press either 'y' or 'n'")
	}

	return false
}

func printVersion() {
	v := version.CurrentVersionNumber
	if version.CurrentCommit != "" {
		v += "-" + version.CurrentCommit
	}
	fmt.Printf("Kubo version: %s\n", v)
	fmt.Printf("Repo version: %d\n", fsrepo.RepoVersion)
	fmt.Printf("System version: %s\n", runtime.GOARCH+"/"+runtime.GOOS)
	fmt.Printf("Golang version: %s\n", runtime.Version())
}

func startVersionChecker(ctx context.Context, nd *core.IpfsNode, enabled bool, percentThreshold int64) {
	if !enabled {
		return
	}
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	go func() {
		for {
			o, err := commands.DetectNewKuboVersion(nd, percentThreshold)
			if err != nil {
				// The version check is best-effort, and may fail in custom
				// configurations that do not run standard WAN DHT. If it
				// errors here, no point in spamming logs: og once and exit.
				log.Errorw("initial version check failed, will not be run again", "error", err)
				return
			}
			if o.UpdateAvailable {
				newerPercent := fmt.Sprintf("%.0f%%", math.Round(float64(o.WithGreaterVersion)/float64(o.PeersSampled)*100))
				log.Errorf(`
⚠️ A NEW VERSION OF KUBO DETECTED

This Kubo node is running an outdated version (%s).
%s of the sampled Kubo peers are running a higher version.
Visit https://github.com/ipfs/kubo/releases or https://dist.ipfs.tech/#kubo and update to version %s or later.`,
					o.RunningVersion, newerPercent, o.GreatestVersion)
			}
			select {
			case <-ctx.Done():
				return
			case <-nd.Process.Closing():
				return
			case <-ticker.C:
				continue
			}
		}
	}()
}
