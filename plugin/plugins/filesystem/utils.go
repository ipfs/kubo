package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/multiformats/go-multiaddr"
)

const (
	PluginName    = "filesystem"
	PluginVersion = "0.0.1"

	//TODO [config]: move elsewhere; related: https://github.com/ipfs/go-ipfs/issues/6526
	EnvAddr = "$IPFS_FS_ADDR" // multiaddr string

	defaultService = "9p" // (currently 9P2000.L)
	sockName       = "filesystem." + defaultService + ".sock"

	tmplHome = "IPFS_HOME"
)

type Config struct { // NOTE: unstable/experimental
	// addresses for file system servers and clients
	//e.g. "9p":"/ip4/localhost/tcp/564", "fuse":"/mountpoint", "🐇":"/rabbit-hutch/glenda", ...
	Service map[string]string
}

func defaultConfig() *Config {
	return &Config{
		map[string]string{
			defaultService: fmt.Sprintf("/unix/${%s}/%s", tmplHome, sockName),
		},
	}
}

func configVarMapper(repoPath string) func(string) string {
	return func(s string) string {
		switch s {
		case tmplHome:
			return repoPath
		default:
			return os.Getenv(s)
		}
	}
}

// removeUnixSockets attempts to remove all unix domain paths from a multiaddr
// does not stop on error, returns last encountered error, except "not exist" errors
func removeUnixSockets(ma multiaddr.Multiaddr) error {
	var retErr error
	multiaddr.ForEach(ma, func(comp multiaddr.Component) bool {
		if comp.Protocol().Code == multiaddr.P_UNIX {
			localPath := filepath.FromSlash(strings.TrimPrefix(comp.String(), "/unix"))
			if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
				retErr = err
			}
		}
		return false
	})
	return retErr
}
