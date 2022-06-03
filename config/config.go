// package config implements the ipfs config file datastructures and utilities.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ipfs/go-ipfs/repo/common"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
)

// Config is the configuration used by an IPFS node.
// NOTE: It is a system-defined configuration with internal defaults that can be
// overridden by a user-defined JSON file (which is *not* the configuration itself).
// FIXME: Currently internal to the Repo interface, it should have its own place
//  in the IpfsNode structure to decouple it from the user file. Similarly,
//  everything here related to the user overrides should its own file.
type Config struct {
	Identity  Identity  // local node's peer identity
	Profiles  string    // profile from the user overrides to apply to defaults
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

// UserConfigOverrides is the Go map representing the loaded JSON file
// with user-defined configuration overrides. It is guaranteed on load
// that its structure matches a subset of the encoded Config struct.
type UserConfigOverrides map[string]interface{}

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
// * If the user-provided configuration file path is empty, use the default one.
// * If the configuration root directory is empty, use the default one.
// * If the user-provided configuration file path is only a file name, use the
//   configuration root directory, otherwise use only the user-provided path
//   and ignore the configuration root.
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
	return Marshal(value)
}

// Marshal configuration with JSON
func Marshal(value interface{}) ([]byte, error) {
	// need to prettyprint, hence MarshalIndent, instead of Encoder
	return json.MarshalIndent(value, "", "  ")
}

func FromMap(v map[string]interface{}) (*Config, error) {
	buf, err := JsonEncode(v)
	if err != nil {
		return nil, err
	}
	return jsonDecodeConfig(buf)
}

func ToMap(conf *Config) (map[string]interface{}, error) {
	buf, err := JsonEncode(conf)
	if err != nil {
		return nil, err
	}
	return jsonDecodeMap(buf)
}

// Clone copies the config. Use when updating.
// FIXME: This can't error. Refactor API.
func (c *Config) Clone() (*Config, error) {
	buf, err := JsonEncode(c)
	if err != nil {
		return nil, err
	}
	return jsonDecodeConfig(buf)
}

func JsonEncode(v interface{}) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		return nil, fmt.Errorf("failure to encode config: %s", err)
	}
	return &buf, nil
}

func jsonDecodeConfig(buf *bytes.Buffer) (*Config, error) {
	var c Config
	if err := json.NewDecoder(buf).Decode(&c); err != nil {
		return nil, fmt.Errorf("failure to decode config: %s", err)
	}
	return &c, nil
}

func jsonDecodeMap(buf *bytes.Buffer) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := json.NewDecoder(buf).Decode(&m); err != nil {
		return nil, fmt.Errorf("failure to decode config: %s", err)
	}
	return m, nil
}

func NewUserConfigOverrides(identity Identity) (UserConfigOverrides, error) {
	// FIXME: Is there an easier way to encode the Identity in a map?
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(identity); err != nil {
		return nil, fmt.Errorf("failure to encode identity: %s", err)
	}
	var m map[string]interface{}
	if err := json.NewDecoder(&buf).Decode(&m); err != nil {
		return nil, fmt.Errorf("failure to decode identity: %s", err)
	}

	return map[string]interface{}{
		"Identity": m,
	}, nil
}

func NewUserConfigOverridesWithProfiles(identity Identity, profiles string) (UserConfigOverrides, error) {
	overrides, err := NewUserConfigOverrides(identity)
	if err != nil {
		return nil, err
	}
	err = CheckProfiles(profiles)
	if err != nil {
		return nil, err
	}
	overrides["Profiles"] = profiles
	return overrides, nil
}

// OverrideMap replaces keys in left map with the right map, recursively traversing
// child maps until a non-map value is found.
// NOTE: Used for the JSON Config-to-map conversions: maps are expected to have
// same types, otherwise this will panic.
func OverrideMap(left, right map[string]interface{}) {
	for key, rightVal := range right {
		leftVal, found := left[key]
		if !found {
			// FIXME: For now nonexistent values in the left will be accepted
			//  and created from right. This is because JSON-decoded default config, left,
			//  still has a lot of `json:",omitempty"` that won't be present. In the future
			//  this should be removed.
			left[key] = rightVal
			continue
		}
		leftMap, ok := leftVal.(map[string]interface{})
		if !ok {
			left[key] = rightVal
			continue
		}
		if rightVal == nil {
			return // FIXME: Do we want to clear config values?
			// If override is empty we should error when loading the user override
			// config file.
		}
		OverrideMap(leftMap, rightVal.(map[string]interface{}))
	}
}

// openConfig returns an error if the config file is not present.
func GetConfig(configFilePath string) (*Config, error) {
	overrides, err := ReadUserConfigOverrides(configFilePath)
	if err != nil {
		return nil, err
	}
	var profiles string
	p, err := common.MapGetKV(overrides, "Profiles")
	if err == nil {
		if profString, ok := p.(string); ok {
			profiles = profString
		}
	}

	defaultConfig, err := DefaultConfig(profiles)
	if err != nil {
		return nil, err
	}

	configMap, err := ToMap(defaultConfig)
	if err != nil {
		return nil, err
	}
	// This shoudln't be neccessary but just in case remove Identity, we'll never
	//  use a default one here.
	delete(configMap, "Identity")

	OverrideMap(configMap, overrides)
	config, err := FromMap(configMap)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func ReadUserConfigOverrides(filename string) (UserConfigOverrides, error) {
	f, err := os.Open(filename)
	if err != nil {
		//if os.IsNotExist(err) {
		//	err = ErrNotInitialized
		//}
		return nil, err
	}
	defer f.Close()

	return DecodeUserConfigOverrides(f)
}

func DecodeUserConfigOverrides(r io.Reader) (UserConfigOverrides, error) {
	var overrides UserConfigOverrides
	dec := json.NewDecoder(r)
	// FIXME: Check that this matches the contents of the Config struct.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&overrides); err != nil {
		return nil, fmt.Errorf("failure to decode user config: %s", err)
	}
	return overrides, nil
}
