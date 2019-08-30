package filesystem

import (
	gopath "path"
	"path/filepath"
	"runtime"
)

const (
	PluginName    = "filesystem"
	PluginVersion = "0.0.1"

	//TODO [config]: move elsewhere; related: https://github.com/ipfs/go-ipfs/issues/6526
	EnvAddr = "IPFS_FS_ADDR" // multiaddr string

	DefaultVersion       = "9P2000.L"
	DefaultListenAddress = "/unix/$IPFS_PATH/" + sockName
	DefaultService       = "9P" // (currently 9P2000.L)
	DefaultMSize         = 64 << 10
	// TODO: For platforms that don't support UDS (Windows < 17063, non-posix systems), fallback to TCP
	//FallbackListenAddress = "/ip4/localhost/tcp/564"

	sockName = "filesystem.9P.sock"
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

	serviceMap[DefaultService] = gopath.Join("/unix", sockTarget)
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
