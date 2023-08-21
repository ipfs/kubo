package fsrepo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ipfs/kubo/config"

	"github.com/facebookgo/atomicfile"
)

// ErrNotInitialized is returned when we fail to read the config because the
// repo doesn't exist.
var ErrNotInitialized = errors.New("ipfs not initialized, please run 'ipfs init'")

// ReadConfigFile reads the config from `filename` into `cfg`.
func ReadConfigFile(filename string, cfg interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			err = ErrNotInitialized
		}
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		return fmt.Errorf("failure to decode config: %w", err)
	}
	return nil
}

// WriteConfigFile writes the config from `cfg` into `filename`.
func WriteConfigFile(filename string, cfg interface{}) error {
	err := os.MkdirAll(filepath.Dir(filename), 0o755)
	if err != nil {
		return err
	}

	f, err := atomicfile.New(filename, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	return encode(f, cfg)
}

// encode configuration with JSON.
func encode(w io.Writer, value interface{}) error {
	// need to prettyprint, hence MarshalIndent, instead of Encoder
	buf, err := config.Marshal(value)
	if err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}

// Load reads given file and returns the read config, or error.
func Load(filename string) (*config.Config, error) {
	var cfg config.Config
	err := ReadConfigFile(filename, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, err
}
