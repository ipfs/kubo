package filesystem

import (
	"context"
	"os"
	"path/filepath"

	"github.com/djdv/p9/p9"
	plugin "github.com/ipfs/go-ipfs/plugin"
	fsnodes "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes"
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
	errorChan chan error
}

func (*FileSystemPlugin) Name() string {
	return PluginName
}

func (*FileSystemPlugin) Version() string {
	return PluginVersion
}

func (fs *FileSystemPlugin) Init(env *plugin.Environment) error {
	logger.Info("Initializing 9P resource server...")

	// stabilise repo path
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
	addrString = os.Expand(addrString, configVarMapper(env.Repo))
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

	// make sure sockets are not in use already (if we're using them)
	err := removeUnixSockets(fs.addr)
	if err != nil {
		return err
	}

	fs.ctx, fs.cancel = context.WithCancel(context.Background())
	fs.errorChan = make(chan error, 1)

	// launch the listener
	listener, err := manet.Listen(fs.addr)
	if err != nil {
		logger.Errorf("9P listen error: %s\n", err)
		return err
	}
	fs.listener = listener

	// construct and launch the 9P resource server
	s := p9.NewServer(fsnodes.RootAttacher(fs.ctx, core, nil))
	go func() {
		fs.errorChan <- s.Serve(manet.NetListener(fs.listener))
	}()

	logger.Infof("9P service is listening on %s\n", fs.listener.Addr())
	return nil
}

func (fs *FileSystemPlugin) Close() error {
	logger.Info("9P server requested to close")
	fs.listener.Close()
	fs.cancel()
	return <-fs.errorChan
}
