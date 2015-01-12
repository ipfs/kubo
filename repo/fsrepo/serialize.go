package fsrepo

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jbenet/go-ipfs/repo/config"
	"github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/debugerror"
)

var log = util.Logger("fsrepo")

// ReadConfigFile reads the config from `filename` into `cfg`.
func ReadConfigFile(filename string, cfg interface{}) error {
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

// WriteConfigFile writes the config from `cfg` into `filename`.
func WriteConfigFile(filename string, cfg interface{}) error {
	err := os.MkdirAll(filepath.Dir(filename), 0775)
	if err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return Encode(f, cfg)
}

// WriteFile writes the buffer at filename
func WriteFile(filename string, buf []byte) error {
	err := os.MkdirAll(filepath.Dir(filename), 0775)
	if err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(buf)
	return err
}

// Encode configuration with JSON
func Encode(w io.Writer, value interface{}) error {
	// need to prettyprint, hence MarshalIndent, instead of Encoder
	buf, err := config.Marshal(value)
	if err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}

// ReadConfigKey retrieves only the value of a particular key
func ReadConfigKey(filename, key string) (interface{}, error) {
	var cfg interface{}
	if err := ReadConfigFile(filename, &cfg); err != nil {
		return nil, err
	}

	var ok bool
	cursor := cfg
	parts := strings.Split(key, ".")
	for i, part := range parts {
		cursor, ok = cursor.(map[string]interface{})[part]
		if !ok {
			sofar := strings.Join(parts[:i], ".")
			return nil, fmt.Errorf("%s key has no attributes", sofar)
		}
	}
	return cursor, nil
}

// WriteConfigKey writes the value of a particular key
func WriteConfigKey(filename, key string, value interface{}) error {
	var cfg interface{}
	if err := ReadConfigFile(filename, &cfg); err != nil {
		return err
	}

	var ok bool
	var mcursor map[string]interface{}
	cursor := cfg

	parts := strings.Split(key, ".")
	for i, part := range parts {
		mcursor, ok = cursor.(map[string]interface{})
		if !ok {
			sofar := strings.Join(parts[:i], ".")
			return fmt.Errorf("%s key is not a map", sofar)
		}

		// last part? set here
		if i == (len(parts) - 1) {
			mcursor[part] = value
			break
		}

		cursor, ok = mcursor[part]
		if !ok { // create map if this is empty
			mcursor[part] = map[string]interface{}{}
			cursor = mcursor[part]
		}
	}

	return WriteConfigFile(filename, cfg)
}

// Load reads given file and returns the read config, or error.
func Load(filename string) (*config.Config, error) {
	// if nothing is there, fail. User must run 'ipfs init'
	if !util.FileExists(filename) {
		return nil, debugerror.New("ipfs not initialized, please run 'ipfs init'")
	}

	var cfg config.Config
	err := ReadConfigFile(filename, &cfg)
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

// Set sets the value of a particular config key
func Set(filename, key, value string) error {
	return WriteConfigKey(filename, key, value)
}

// RecordUpdateCheck is called to record that an update check was performed,
// showing that the running version is the most recent one.
func RecordUpdateCheck(cfg *config.Config, filename string) {
	cfg.Version.CheckDate = time.Now()

	if cfg.Version.CheckPeriod == "" {
		// CheckPeriod was not initialized for some reason (e.g. config file broken)
		log.Error("config.Version.CheckPeriod not set. config broken?")
	}

	WriteConfigFile(filename, cfg)
}
