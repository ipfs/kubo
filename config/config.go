// package config implements the ipfs config file datastructures and utilities.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/facebookgo/atomicfile"
	"github.com/mitchellh/go-homedir"
)

// ErrNotInitialized is returned when we fail to read the config because the
// repo doesn't exist.
var ErrNotInitialized = errors.New("ipfs not initialized, please run 'ipfs init'")

// Config is used to load ipfs config files.
type Config struct {
	Identity  Identity  // local node's peer identity
	Datastore Datastore // local node's storage
	Addresses Addresses // local node's addresses
	Mounts    Mounts    // local node's mount points
	Discovery Discovery // local node's discovery mechanisms
	Routing   Routing   // local node's routing settings
	Ipns      Ipns      // Ipns settings
	Bootstrap []string  // local nodes's bootstrap peer addresses
	Gateway   Gateway   // local node's gateway server options
	API       API       // local node's API settings
	Swarm     SwarmConfig
	AutoNAT   AutoNATConfig
	Pubsub    PubsubConfig
	Peering   Peering
	DNS       DNS
	Migration Migration

	Provider     Provider
	Reprovider   Reprovider
	Experimental Experiments
	Plugins      Plugins
	Pinning      Pinning

	Internal Internal // experimental/unstable options
}

const (
	// DefaultPathName is the default config dir name
	DefaultPathName = ".ipfs"
	// DefaultPathRoot is the path to the default config dir location.
	DefaultPathRoot = "~/" + DefaultPathName
	// DefaultConfigFile is the filename of the configuration file
	DefaultConfigFile = "config"
	// EnvDir is the environment variable used to change the path root.
	EnvDir = "IPFS_PATH"
)

// PathRoot returns the default configuration root directory
func PathRoot() (string, error) {
	dir := os.Getenv(EnvDir)
	var err error
	if len(dir) == 0 {
		dir, err = homedir.Expand(DefaultPathRoot)
	}
	return dir, err
}

// Path returns the path `extension` relative to the configuration root. If an
// empty string is provided for `configroot`, the default root is used.
func Path(configroot, extension string) (string, error) {
	if len(configroot) == 0 {
		dir, err := PathRoot()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, extension), nil

	}
	return filepath.Join(configroot, extension), nil
}

// Filename returns the configuration file path given a configuration root
// directory and a user-provided configuration file path argument with the
// following rules:
//   - If the user-provided configuration file path is empty, use the default one.
//   - If the configuration root directory is empty, use the default one.
//   - If the user-provided configuration file path is only a file name, use the
//     configuration root directory, otherwise use only the user-provided path
//     and ignore the configuration root.
func Filename(configroot string, userConfigFile string) (string, error) {
	if userConfigFile == "" {
		return Path(configroot, DefaultConfigFile)
	}

	if filepath.Dir(userConfigFile) == "." {
		return Path(configroot, userConfigFile)
	}

	return userConfigFile, nil
}

// HumanOutput gets a config value ready for printing
func HumanOutput(value interface{}) ([]byte, error) {
	s, ok := value.(string)
	if ok {
		return []byte(strings.Trim(s, "\n")), nil
	}

	buf := new(bytes.Buffer)
	if err := encodeConfigFile(buf, value); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func FromMap(v map[string]interface{}) (*Config, error) {
	buf := new(bytes.Buffer)

	if err := encodeConfigFile(buf, v); err != nil {
		return nil, err
	}
	var conf Config
	if err := decodeConfigFile(buf, &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}

func ToMap(conf *Config) (map[string]interface{}, error) {
	buf := new(bytes.Buffer)
	if err := encodeConfigFile(buf, conf); err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := decodeConfigFile(buf, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// Clone copies the config. Use when updating.
func (c *Config) Clone() (*Config, error) {
	var newConfig Config
	var buf bytes.Buffer

	if err := encodeConfigFile(&buf, c); err != nil {
		return nil, err
	}
	if err := decodeConfigFile(&buf, &newConfig); err != nil {
		return nil, err
	}

	return &newConfig, nil
}

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

	return decodeConfigFile(f, cfg)
}

func decodeConfigFile(r io.Reader, cfg interface{}) error {
	dec := json.NewDecoder(r)
	if os.Getenv("IPFS_CONFIG_TOLERANT_MODE") == "" {
		dec.DisallowUnknownFields()
	}

	if err := dec.Decode(cfg); err != nil {
		return fmt.Errorf("failure to decode config: %s", err)
	}

	return nil
}

// WriteConfigFile writes the config from `cfg` into `filename`.
func WriteConfigFile(filename string, cfg interface{}) error {
	err := os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return err
	}

	f, err := atomicfile.New(filename, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return encodeConfigFile(f, cfg)
}

// encodeConfigFile encodes configuration with JSON
func encodeConfigFile(w io.Writer, value interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")

	return enc.Encode(value)
}

// Load reads given file and returns the read config, or error.
func Load(filename string) (*Config, error) {
	var cfg Config
	err := ReadConfigFile(filename, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, err
}
