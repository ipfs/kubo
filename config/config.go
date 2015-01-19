// package config implements the ipfs config file datastructures and utilities.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("config")

// Config is used to load IPFS config files.
type Config struct {
	Identity  Identity        // local node's peer identity
	Datastore Datastore       // local node's storage
	Addresses Addresses       // local node's addresses
	Mounts    Mounts          // local node's mount points
	Version   Version         // local node's version management
	Bootstrap []BootstrapPeer // local nodes's bootstrap peers
	Tour      Tour            // local node's tour position
	Logs      Logs            // local node's event log configuration
}

const (
	// DefaultPathRoot is the path to the default config dir location.
	DefaultPathRoot = "~/.go-ipfs"
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
		dir, err = u.TildeExpansion(DefaultPathRoot)
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
// directory. If the configuration root directory is empty, use the default one
func Filename(configroot string) (string, error) {
	return Path(configroot, DefaultConfigFile)
}

// HumanOutput gets a config value ready for printing
func HumanOutput(value interface{}) ([]byte, error) {
	s, ok := value.(string)
	if ok {
		return []byte(strings.Trim(s, "\n")), nil
	}
	return Marshal(value)
}

// Marshal configuration with JSON
func Marshal(value interface{}) ([]byte, error) {
	// need to prettyprint, hence MarshalIndent, instead of Encoder
	return json.MarshalIndent(value, "", "  ")
}

func FromMap(v map[string]interface{}) (*Config, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	var conf Config
	if err := json.NewDecoder(&buf).Decode(&conf); err != nil {
		return nil, fmt.Errorf("Failure to decode config: %s", err)
	}
	return &conf, nil
}

func ToMap(conf *Config) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(conf); err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.NewDecoder(&buf).Decode(&m); err != nil {
		return nil, fmt.Errorf("Failure to decode config: %s", err)
	}
	return m, nil
}
