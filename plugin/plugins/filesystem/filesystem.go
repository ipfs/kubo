package filesystem

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/hugelgupf/p9/p9"
	plugin "github.com/ipfs/go-ipfs/plugin"
	fsnodes "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/mitchellh/mapstructure"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

var (
	_ plugin.PluginDaemon = (*FileSystemPlugin)(nil) // impl check

	// Plugins is an exported list of plugins that will be loaded by go-ipfs.
	Plugins = []plugin.Plugin{
		&FileSystemPlugin{}, //TODO: individually name implementations: &P9{}
	}

	logger logging.EventLogger
)

func init() {
	logger = logging.Logger("plugin/filesystem")
}

type FileSystemPlugin struct {
	ctx    context.Context
	cancel context.CancelFunc

	addr      multiaddr.Multiaddr
	listener  manet.Listener
	closed    chan struct{}
	serverErr error
}

func (*FileSystemPlugin) Name() string {
	return PluginName
}

func (*FileSystemPlugin) Version() string {
	return PluginVersion
}

func (fs *FileSystemPlugin) Init(env *plugin.Environment) error {
	logger.Info("Initializing 9P resource server...")
	if fs.addr != nil {
		return fmt.Errorf("already initialized with %s", fs.addr.String())
	}

	// stabilise repo path; our template depends on this
	if !filepath.IsAbs(env.Repo) {
		absRepo, err := filepath.Abs(env.Repo)
		if err != nil {
			return err
		}
		env.Repo = absRepo
	}

	cfg := &Config{}
	// load config from file or initialize it
	if env.Config != nil {
		if err := mapstructure.Decode(env.Config, cfg); err != nil {
			return err
		}
	} else {
		cfg = defaultConfig()
	}

	var addrString string
	// allow environment variable to override config values
	if envAddr := os.ExpandEnv(EnvAddr); envAddr != "" {
		addrString = EnvAddr
	} else {
		addrString = cfg.Service[defaultService]
	}

	// expand string templates and initialize listening addr
	templateRepoPath := env.Repo
	if strings.HasPrefix(addrString, "/unix") {
		// prevent template from expanding to double slashed paths like `/unix//home/...`
		// but allow it to expand to `/unix/C:\Users...`
		templateRepoPath = strings.TrimPrefix(templateRepoPath, "/")
	}

	addrString = os.Expand(addrString, configVarMapper(templateRepoPath))

	ma, err := multiaddr.NewMultiaddr(addrString)
	if err != nil {
		return err
	}
	fs.addr = ma

	logger.Info("9P resource server okay for launch")
	return nil
}

func (fs *FileSystemPlugin) Start(core coreiface.CoreAPI) error {
	logger.Info("Starting 9P resource server...")
	if fs.addr == nil {
		return fmt.Errorf("Start called before plugin Init")
	}

	// make sure we're not in use already
	if fs.listener != nil {
		return fmt.Errorf("already started and listening on %s", fs.listener.Addr())
	}

	// make sure sockets are not in use already (if we're using them)
	err := removeUnixSockets(fs.addr)
	if err != nil {
		return err
	}

	// launch the listener
	listener, err := manet.Listen(fs.addr)
	if err != nil {
		logger.Errorf("9P listen error: %s\n", err)
		return err
	}
	fs.listener = listener

	// construct and launch the 9P resource server
	fs.ctx, fs.cancel = context.WithCancel(context.Background())
	fs.closed = make(chan struct{})

	opts := []nodeopts.AttachOption{nodeopts.Logger(logging.Logger("9root"))}
	server := p9.NewServer(fsnodes.RootAttacher(fs.ctx, core, opts...))
	go func() {
		// run the server until the listener closes
		// store error on the fs object then close our syncing channel (see use in `Close` below)
		fs.serverErr = server.Serve(manet.NetListener(fs.listener))

		if fs.ctx.Err() != nil { // [async] we expect `Accept` to fail only if the filesystem is canceled
			var opErr *net.OpError
			if errors.As(fs.serverErr, &opErr) && opErr.Op == "accept" {
				fs.serverErr = nil
			}
		}

		close(fs.closed)
	}()

	logger.Infof("9P service is listening on %s\n", fs.listener.Addr())
	return nil
}

func (fs *FileSystemPlugin) Close() error {
	logger.Info("9P server requested to close")
	if fs.addr == nil { // forbidden
		return fmt.Errorf("Close called before plugin Init")
	}

	// synchronization between plugin interface <-> fs server
	if fs.closed != nil { // implies `Start` was called prior
		fs.cancel()         // stop and prevent all fs operations, signifies "closing" intent
		fs.listener.Close() // stop accepting new clients
		<-fs.closed         // wait for the server thread to set the error value
		fs.listener = nil   // reset `Start` conditions
		fs.closed = nil
	}
	// otherwise we were never started to begin with; default/initial value will be returned
	return fs.serverErr
}
