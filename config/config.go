package config

import (
	u "github.com/jbenet/go-ipfs/util"
	"os"
)

// Identity tracks the configuration of the local node's identity.
type Identity struct {
	Curve uint16
}

// Datastore tracks the configuration of the datastore.
type Datastore struct {
	Type string
	Path string
}

// Config is used to load IPFS config files.
type Config struct {
	Identity  *Identity
	Datastore *Datastore
}

var defaultConfigFilePath = "~/.go-ipfs/config"
var defaultConfigFile = `{
  "identity": {
    "curve": 224
  },
  "datastore": {
    "type": "leveldb",
    "path": "~/.go-ipfs/datastore"
  }
}
`

// Filename returns the proper tilde expanded config filename.
func Filename(filename string) (string, error) {
	if len(filename) == 0 {
		filename = defaultConfigFilePath
	}

	// tilde expansion on config file
	return u.TildeExpansion(filename)
}

// Load reads given file and returns the read config, or error.
func Load(filename string) (*Config, error) {
	filename, err := Filename(filename)
	if err != nil {
		return nil, err
	}

	// if nothing is there, write first config file.
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		WriteFile(filename, []byte(defaultConfigFile))
	}

	var cfg Config
	err = ReadConfigFile(filename, &cfg)
	if err != nil {
		return nil, err
	}

	// tilde expansion on datastore path
	cfg.Datastore.Path, err = u.TildeExpansion(cfg.Datastore.Path)
	if err != nil {
		return nil, err
	}

	return &cfg, err
}

// Set sets the value of a particular config key
func Set(filename, key, value string) error {
	return nil
}
