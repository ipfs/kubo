package filesystem

import (
	"os"
	gopath "path"
	"path/filepath"
	"runtime"

	config "github.com/ipfs/go-ipfs-config"
	cserial "github.com/ipfs/go-ipfs-config/serialize"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

const (
	//TODO [config]: move elsewhere; related: https://github.com/ipfs/go-ipfs/issues/6526
	EnvAddr = "IPFS_FS_ADDR" // multiaddr string

	DefaultVersion       = "9P2000.L"
	DefaultListenAddress = "/unix/$IPFS_PATH/filesystem.9p.sock"
	DefaultService       = "9P" // (currently 9P2000.L)
	DefaultMSize         = 64 << 10
	// TODO: For platforms that don't support UDS (Windows < 17063, non-posix systems), fallback to TCP
	//FallbackListenAddress = "/ip4/localhost/tcp/564"
)

/*
func defaultConfig() (*config.FileSystem, error) {
	fsConf := make(map[string]string)
	target, err := config.Path("", "filesystem.9p.sock")
	if err != nil {
		return nil, err
	}
	fsConf["9p"] = gopath.Join("/unix", target)
	return fsConf, nil
}
*/

func getConfig() (*config.Config, error) {
	//TODO [plugin]: default config init+file should go somewhere else
	// preferably we'd have plugin-scoped storage somehow
	//$IPFS_PATH/plugins/filesystem/{sock, conf} or `plugin.Start(core, pluginCfg`) ?
	cfgPath, err := config.Filename("")
	if err != nil {
		return nil, err
	}

	return cserial.Load(cfgPath)
}

func GetFSConf() (*config.FileSystem, error) {
	cfg, err := getConfig()
	if err != nil {
		return nil, err
	}

	/* We can't actually enable this yet 👀
	if !cfg.Experimental.FileSystemEnabled {
		return nil, errDisabled
	}
	*/

	if addr := os.Getenv(EnvAddr); addr != "" {
		cfg.FileSystem.Service[DefaultService] = addr
		return &cfg.FileSystem, nil
	}

	//TODO: after experiment, make sure this is populated from conf file, not initialised here
	if cfg.FileSystem.Service == nil {
		cfg.FileSystem.Service = make(map[string]string)
	}

	if comp := cfg.FileSystem.Service[DefaultService]; comp == DefaultListenAddress || comp == "" {
		// expand $IPFS_PATH, using default if not exist
		target, err := config.Path("", "filesystem.9p.sock")
		if err != nil {
			return nil, err
		}

		if runtime.GOOS == "windows" {
			if target, err = windowsToUnixFriendly(target); err != nil {
				return nil, err
			}
		}
		cfg.FileSystem.Service[DefaultService] = gopath.Join("/unix", target)
	} else {
		// assume user supplied env vars exists and expand them as-is
		for service, target := range cfg.FileSystem.Service {
			cfg.FileSystem.Service[service] = os.ExpandEnv(target)
		}
	}

	return &cfg.FileSystem, nil
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

func GetListener() (manet.Listener, error) {
	cfg, err := GetFSConf()
	if err != nil {
		return nil, err
	}

	ma, err := multiaddr.NewMultiaddr(cfg.Service[DefaultService])
	if err != nil {
		return nil, err
	}

	listenAddress, err := manet.Listen(ma)
	if err != nil {
		return nil, err
	}
	return listenAddress, nil
}
