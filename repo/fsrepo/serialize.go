package fsrepo

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jbenet/go-ipfs/repo/config"
	"github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/debugerror"
)

var log = util.Logger("fsrepo")

// readConfigFile reads the config from `filename` into `cfg`.
func readConfigFile(filename string, cfg interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		return fmt.Errorf("Failure to decode config: %s", err)
	}
	return nil
}

// writeConfigFile writes the config from `cfg` into `filename`.
func writeConfigFile(filename string, cfg interface{}) error {
	err := os.MkdirAll(filepath.Dir(filename), 0775)
	if err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return encode(f, cfg)
}

// encode configuration with JSON
func encode(w io.Writer, value interface{}) error {
	// need to prettyprint, hence MarshalIndent, instead of Encoder
	buf, err := config.Marshal(value)
	if err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}

// load reads given file and returns the read config, or error.
func load(filename string) (*config.Config, error) {
	// if nothing is there, fail. User must run 'ipfs init'
	if !util.FileExists(filename) {
		return nil, debugerror.New("ipfs not initialized, please run 'ipfs init'")
	}

	var cfg config.Config
	err := readConfigFile(filename, &cfg)
	if err != nil {
		return nil, err
	}

	// tilde expansion on datastore path
	cfg.Datastore.Path, err = util.TildeExpansion(cfg.Datastore.Path)
	if err != nil {
		return nil, err
	}

	return &cfg, err
}
