package fsrepo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	common "github.com/jbenet/go-ipfs/repo/common"
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

// GetConfigKey retrieves only the value of a particular key
func (r *FSRepo) GetConfigKey(key string) (interface{}, error) {
	filename, err := config.Filename(r.path)
	if err != nil {
		return nil, err
	}
	var cfg map[string]interface{}
	if err := ReadConfigFile(filename, &cfg); err != nil {
		return nil, err
	}

	return common.MapGetKV(cfg, key)
}

// SetConfigKey writes the value of a particular key
func (r *FSRepo) SetConfigKey(key string, value interface{}) error {
	filename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	var mapconf map[string]interface{}
	if err := ReadConfigFile(filename, &mapconf); err != nil {
		return err
	}
	if err := common.MapSetKV(mapconf, key, value); err != nil {
		return err
	}
	// must use raw method because there may exist keys not present in the *config.Config struct
	if err := writeConfigFile(filename, mapconf); err != nil {
		return err
	}
	conf, err := convertMapToConfig(mapconf)
	if err != nil {
		return err
	}
	*r.config = *conf // copy so caller cannot modify the private config
	return nil
}

func convertMapToConfig(v map[string]interface{}) (*config.Config, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	var conf config.Config
	if err := json.NewDecoder(&buf).Decode(&conf); err != nil {
		return nil, fmt.Errorf("Failure to decode config: %s", err)
	}
	return &conf, nil
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

// RecordUpdateCheck is called to record that an update check was performed,
// showing that the running version is the most recent one.
//
// DEPRECATED
func RecordUpdateCheck(cfg *config.Config, filename string) {
	cfg.Version.CheckDate = time.Now()

	if cfg.Version.CheckPeriod == "" {
		// CheckPeriod was not initialized for some reason (e.g. config file broken)
		log.Error("config.Version.CheckPeriod not set. config broken?")
	}

	writeConfigFile(filename, cfg)
}
