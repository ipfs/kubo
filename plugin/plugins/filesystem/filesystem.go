package filesystem

import (
	"context"
	"fmt"
	"net"

	"github.com/hugelgupf/p9/p9"
	plugin "github.com/ipfs/go-ipfs/plugin"
	fsnodes "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

// Plugins is an exported list of plugins that will be loaded by go-ipfs.
var Plugins = []plugin.Plugin{
	&FileSystemPlugin{},
}

// impl check
var _ plugin.PluginDaemon = (*FileSystemPlugin)(nil)

type FileSystemPlugin struct {
	ctx    context.Context
	cancel context.CancelFunc
	addr   string //TODO: populate with value the server is listening on
}

func (*FileSystemPlugin) Name() string {
	return "filesystem"
}

func (*FileSystemPlugin) Version() string {
	return "0.0.1"
}

func (fs *FileSystemPlugin) Init() error {
	fs.ctx, fs.cancel = context.WithCancel(context.Background())
	return nil
}

func init() {
}

func (fs *FileSystemPlugin) Start(core coreiface.CoreAPI) error {
	logger := logging.Logger("plugin/filesystem")
	logger.Info("Initialising 9p resource server...")

	// construct 9p resource server / config
	proto, addr, err := getAddr()
	if err != nil {
		logger.Errorf("9P server error: %s", err)
		return err
	}

	p9pFSS, err := fsnodes.NewRoot(fs.ctx, core, logger)
	if err != nil {
		logger.Errorf("9P server error: %s", err)
		return err
	}

	// Bind and listen on the socket.
	serverSocket, err := net.Listen(proto, addr)
	if err != nil {
		logger.Errorf("9P server error: %s", err)
		return err
	}

	// Run the server.
	s := p9.NewServer(p9pFSS)
	go func() {
		if err := s.Serve(serverSocket); err != nil {
			logger.Errorf("9P server error: %s", err)
			return
		}
	}()

	//TODO: prettier print; mountpoints, socket/addr, confirm start actually succeeded, etc.
	logger.Infof("9P service started on %s", addr)
	return nil
}

func (fs *FileSystemPlugin) Close() error {
	//TODO: fmt.Println("Closing file system handles...")
	fmt.Println("9P server requested to close")
	fs.cancel()
	return nil
}
