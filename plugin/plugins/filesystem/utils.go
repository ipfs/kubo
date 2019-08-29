package filesystem

import (
	"os"
	gopath "path"
	"path/filepath"
	"runtime"

	config "github.com/ipfs/go-ipfs-config"
	cserial "github.com/ipfs/go-ipfs-config/serialize"
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

func defaultConfig() (*Config, error) {
	serviceMap := make(map[string]string)
	target, err := config.Path("", sockName)
	if err != nil {
		return nil, err
	}

	if runtime.GOOS == "windows" {
		if target, err = windowsToUnixFriendly(target); err != nil {
			return nil, err
		}
	}

	serviceMap["9P"] = gopath.Join("/unix", target)
	return &Config{serviceMap}, nil
}

//TODO: better name
func XXX_GetFSConf() (*Config, error) {
	return configFromPlugin(PluginName)
}

func windowsToUnixFriendly(target string) (string, error) {
	if !filepath.IsAbs(target) {
		var err error
		if target, err = filepath.Abs(target); err != nil {
			return target, err
		}
	}

	//TODO [manet]: doesn't like drive letters
	//XXX: so for now we decap drive-spec-like paths and use the current working drive letter, relatively
	if len(target) > 3 && target[1] == ':' {
		target = target[3:]
	}
	return filepath.ToSlash(target), nil
}

func configFromPlugin(pluginName string) (*Config, error) {
	//TODO: after experiment, make sure this is populated from conf file, not initialised here
	cfgPath, err := config.Filename("")
	if err != nil {
		return nil, err
	}

	cfg, err := cserial.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	pluginCfg := cfg.Plugins.Plugins[PluginName]
	if pluginCfg.Disabled {
		return nil, errDisabled
	}

	serviceConfig, ok := pluginCfg.Config.(*Config)
	if !ok {
		if serviceConfig, err = defaultConfig(); err != nil {
			return nil, err
		}
	}

	if addr := os.Getenv(EnvAddr); addr != "" {
		serviceConfig.Service[DefaultService] = addr
	}

	// assume user supplied env vars are set and expand them as-is
	for service, target := range serviceConfig.Service {
		serviceConfig.Service[service] = os.ExpandEnv(target)
	}

	return serviceConfig, nil
}
