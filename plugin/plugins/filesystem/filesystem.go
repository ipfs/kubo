package filesystem

import (
	"context"
	"errors"

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
	ctx      context.Context
	cancel   context.CancelFunc
	listener manet.Listener

	disabled bool
}

func (*FileSystemPlugin) Name() string {
	return PluginName
}

func (*FileSystemPlugin) Version() string {
	return PluginVersion
}

func (fs *FileSystemPlugin) Init() error {
	fs.ctx, fs.cancel = context.WithCancel(context.Background())
	return nil
}

var (
	logger      logging.EventLogger
	errDisabled = errors.New("this experiment is disabled, enable with `ipfs config --json Experimental.FileSystemEnabled true`")
)

func init() {
	logger = logging.Logger("plugin/filesystem")
}

func (fs *FileSystemPlugin) Start(core coreiface.CoreAPI) error {
	logger.Info("Initialising 9p resource server...")
	fs.disabled = true

	serviceConfig, err := XXX_GetFSConf()
	if err != nil {
		if err == errDisabled {
			logger.Warning(errDisabled)
			return nil
		}
		return err
	}

	ma, err := multiaddr.NewMultiaddr(serviceConfig.Service[DefaultService])
	if err != nil {
		logger.Errorf("9P multiaddr error: %s\n", err)
		return err
	}

	if fs.listener, err = manet.Listen(ma); err != nil {
		logger.Errorf("9P listen error: %s\n", err)
		return err
	}

	// construct 9p resource server
	p9pFSS, err := fsnodes.NewRoot(fs.ctx, core, logger)
	if err != nil {
		logger.Errorf("9P construction error: %s\n", err)
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

	fs.disabled = false
	logger.Infof("9P service started on %s\n", fs.listener.Addr())
	return nil
}

func (fs *FileSystemPlugin) Close() error {
	if fs.disabled {
		return nil
	}

	//TODO: fmt.Println("Closing file system handles...")
	logger.Info("9P server requested to close")
	fs.cancel()
	fs.listener.Close()
	return nil
}
