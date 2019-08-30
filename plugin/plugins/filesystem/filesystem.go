package filesystem

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/hugelgupf/p9/p9"
	plugin "github.com/ipfs/go-ipfs/plugin"
	fsnodes "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

// Plugins is an exported list of plugins that will be loaded by go-ipfs.
var Plugins = []plugin.Plugin{
	&FileSystemPlugin{}, //TODO: individually name implementations: &P9{}
}

// impl check
var _ plugin.PluginDaemon = (*FileSystemPlugin)(nil)

type FileSystemPlugin struct {
	ctx    context.Context
	cancel context.CancelFunc

	addr     multiaddr.Multiaddr
	listener manet.Listener
}

func (*FileSystemPlugin) Name() string {
	return PluginName
}

func (*FileSystemPlugin) Version() string {
	return PluginVersion
}

func (fs *FileSystemPlugin) Init(env *plugin.Environment) error {
	logger.Info("Initializing 9P resource server...")
	if !filepath.IsAbs(env.Repo) {
		absRepo, err := filepath.Abs(env.Repo)
		if err != nil {
			return err
		}
		env.Repo = absRepo
	}

	cfg := &Config{}
	if env.Config != nil {
		byteRep, err := json.Marshal(env.Config)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(byteRep, cfg); err != nil {
			return err
		}
	} else {
		cfg = defaultConfig(env.Repo)
	}

	var err error
	fs.addr, err = multiaddr.NewMultiaddr(cfg.Service[DefaultService])
	if err != nil {
		return err
	}

	fs.ctx, fs.cancel = context.WithCancel(context.Background())
	logger.Info("9P resource server okay for launch")
	return nil
}

var (
	logger logging.EventLogger
)

func init() {
	logger = logging.Logger("plugin/filesystem")
}

func (fs *FileSystemPlugin) Start(core coreiface.CoreAPI) error {
	logger.Info("Starting 9P resource server...")

	var err error
	if fs.listener, err = manet.Listen(fs.addr); err != nil {
		logger.Errorf("9P listen error: %s\n", err)
		return err
	}

	// construct 9P resource server
	p9pFSS, err := fsnodes.NewRoot(fs.ctx, core, logger)
	if err != nil {
		logger.Errorf("9P root construction error: %s\n", err)
		return err
	}

	// Run the server.
	s := p9.NewServer(p9pFSS)
	go func() {
		if err := s.Serve(manet.NetListener(fs.listener)); err != nil {
			logger.Errorf("9P server error: %s\n", err)
			return
		}
	}()

	logger.Infof("9P service is listening on %s\n", fs.listener.Addr())
	return nil
}

func (fs *FileSystemPlugin) Close() error {
	//TODO: fmt.Println("Closing file system handles...")
	logger.Info("9P server requested to close")
	fs.cancel()
	fs.listener.Close()
	return nil
}
