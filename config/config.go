package config

import (
	"os"
	u "github.com/jbenet/go-ipfs/util"
)

type Identity struct {
	PeerId string
}

type Datastore struct {
	Type string
	Path string
}

type Config struct {
	Identity  *Identity
	Datastore *Datastore
}

var defaultConfigFilePath = "~/.go-ipfs/config"
var defaultConfigFile = `{
  "identity": {},
  "datastore": {
    "type": "leveldb",
    "path": "~/.go-ipfs/datastore"
  }
}
`

func LoadConfig(filename string) (*Config, error) {
	if len(filename) == 0 {
		filename = defaultConfigFilePath
	}

	// tilde expansion on config file
	filename, err := u.TildeExpansion(filename)
	if err != nil {
		return nil, err
	}

	// if nothing is there, write first conifg file.
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
