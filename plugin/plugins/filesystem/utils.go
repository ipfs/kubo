package filesystem

import (
	"os"
	gopath "path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/multiformats/go-multiaddr"
)

const (
	PluginName    = "filesystem"
	PluginVersion = "0.0.1"

	//TODO [config]: move elsewhere; related: https://github.com/ipfs/go-ipfs/issues/6526
	EnvAddr = "IPFS_FS_ADDR" // multiaddr string

	sockName       = "filesystem.9P.sock"
	defaultService = "9P" // (currently 9P2000.L)
)

type Config struct { // NOTE: unstable/experimental
	// addresses for file system servers and clients
	//e.g. "9P":"/ip4/localhost/tcp/564", "fuse":"/mountpoint", "🐇":"/rabbit-hutch/glenda", ...
	Service map[string]string
}

func defaultConfig(storagePath string) *Config {
	serviceMap := make(map[string]string)

	sockTarget := gopath.Join(storagePath, sockName)
	if runtime.GOOS == "windows" {
		sockTarget = windowsToUnixFriendly(sockTarget)
	}

	serviceMap[defaultService] = gopath.Join("/unix", sockTarget)
	return &Config{serviceMap}
}

func windowsToUnixFriendly(target string) string {
	//TODO [manet]: doesn't like drive letters
	//XXX: so for now we decap drive-spec-like paths and use the current working drive letter, relatively
	if len(target) > 2 && target[1] == ':' {
		target = target[2:]
	}
	return filepath.ToSlash(target)
}

// removeUnixSockets attempts to remove all unix domain paths from a multiaddr
// Does not stop on error, returns last encountered error
func removeUnixSockets(ma multiaddr.Multiaddr) error {
	var retErr error
	multiaddr.ForEach(ma, func(comp multiaddr.Component) bool {
		if comp.Protocol().Code == multiaddr.P_UNIX {
			if err := os.Remove(strings.TrimPrefix(comp.String(), "/unix")); err != nil {
				retErr = err
			}
		}
		return false
	})
	return retErr
}
