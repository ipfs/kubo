package nopfs

import (
	"os"
	"path/filepath"

	"github.com/ipfs-shipyard/nopfs"
	"github.com/ipfs-shipyard/nopfs/ipfs"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node"
	"github.com/ipfs/kubo/plugin"
	"go.uber.org/fx"
)

// Plugins sets the list of plugins to be loaded.
var Plugins = []plugin.Plugin{
	&nopfsPlugin{},
}

// fxtestPlugin is used for testing the fx plugin.
// It merely adds an fx option that logs a debug statement, so we can verify that it works in tests.
type nopfsPlugin struct {
	// Path to the IPFS repo.
	repo string
}

var _ plugin.PluginFx = (*nopfsPlugin)(nil)

func (p *nopfsPlugin) Name() string {
	return "nopfs"
}

func (p *nopfsPlugin) Version() string {
	return "0.0.10"
}

func (p *nopfsPlugin) Init(env *plugin.Environment) error {
	p.repo = env.Repo

	return nil
}

// MakeBlocker is a factory for the blocker so that it can be provided with Fx.
func MakeBlocker(repoPath string) func() (*nopfs.Blocker, error) {
	return func() (*nopfs.Blocker, error) {
		defaultFiles, err := nopfs.GetDenylistFiles()
		if err != nil {
			return nil, err
		}

		kuboFiles, err := nopfs.GetDenylistFilesInDir(filepath.Join(repoPath, "denylists"))
		if err != nil {
			return nil, err
		}

		files := append(defaultFiles, kuboFiles...)

		return nopfs.NewBlocker(files)
	}
}

// PathResolvers returns wrapped PathResolvers for Kubo.
func PathResolvers(fetchers node.FetchersIn, blocker *nopfs.Blocker) node.PathResolversOut {
	res := node.PathResolverConfig(fetchers)
	return node.PathResolversOut{
		IPLDPathResolver:          ipfs.WrapResolver(res.IPLDPathResolver, blocker),
		UnixFSPathResolver:        ipfs.WrapResolver(res.UnixFSPathResolver, blocker),
		OfflineIPLDPathResolver:   ipfs.WrapResolver(res.OfflineIPLDPathResolver, blocker),
		OfflineUnixFSPathResolver: ipfs.WrapResolver(res.OfflineUnixFSPathResolver, blocker),
	}
}

func (p *nopfsPlugin) Options(info core.FXNodeInfo) ([]fx.Option, error) {
	if os.Getenv("IPFS_CONTENT_BLOCKING_DISABLE") != "" {
		return info.FXOptions, nil
	}

	opts := append(
		info.FXOptions,
		fx.Provide(MakeBlocker(p.repo)),
		fx.Decorate(ipfs.WrapBlockService),
		fx.Decorate(ipfs.WrapNameSystem),
		fx.Decorate(PathResolvers),
	)
	return opts, nil
}
