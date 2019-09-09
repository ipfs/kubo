package filesystem

import (
	"fmt"

	plugin "github.com/ipfs/go-ipfs/plugin"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

// Plugins is an exported list of plugins that will be loaded by go-ipfs.
var Plugins = []plugin.Plugin{
	&FileSystemPlugin{},
}

// impl check
var _ plugin.PluginDaemon = (*FileSystemPlugin)(nil)

type FileSystemPlugin struct{}

func (*FileSystemPlugin) Name() string {
	return "filesystem"
}

func (*FileSystemPlugin) Version() string {
	return "0.0.1"
}

func (*FileSystemPlugin) Init() error {
	return nil
}

func (*FileSystemPlugin) Start(_ coreiface.CoreAPI) error {
	fmt.Println("Initialising file system...")
	//TODO: print mountpoints, sockets, etc.
	fmt.Println("FS okay")
	return nil
}

func (*FileSystemPlugin) Close() error {
	fmt.Println("Closing file system handles...")
	return nil
}
