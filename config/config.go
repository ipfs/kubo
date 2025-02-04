// package config implements the ipfs config file datastructures and utilities.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ipfs/kubo/misc/fsutil"
)

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
	AutoTLS   AutoTLS
	Pubsub    PubsubConfig
	Peering   Peering
	DNS       DNS
	Migration Migration

	Provider     Provider
	Reprovider   Reprovider
	Experimental Experiments
	Plugins      Plugins
	Pinning      Pinning
	Import       Import
	Version      Version

	Internal Internal // experimental/unstable options
}

const (
	// DefaultPathName is the default config dir name.
	DefaultPathName = ".ipfs"
	// DefaultPathRoot is the path to the default config dir location.
	DefaultPathRoot = "~/" + DefaultPathName
	// DefaultConfigFile is the filename of the configuration file.
	DefaultConfigFile = "config"
	// EnvDir is the environment variable used to change the path root.
	EnvDir = "IPFS_PATH"
)

// PathRoot returns the default configuration root directory.
func PathRoot() (string, error) {
	dir := os.Getenv(EnvDir)
	var err error
	if len(dir) == 0 {
		dir, err = fsutil.ExpandHome(DefaultPathRoot)
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
func Filename(configroot, userConfigFile string) (string, error) {
	if userConfigFile == "" {
		return Path(configroot, DefaultConfigFile)
	}

	if filepath.Dir(userConfigFile) == "." {
		return Path(configroot, userConfigFile)
	}

	return userConfigFile, nil
}

// HumanOutput gets a config value ready for printing.
func HumanOutput(value interface{}) ([]byte, error) {
	s, ok := value.(string)
	if ok {
		return []byte(strings.Trim(s, "\n")), nil
	}
	return Marshal(value)
}

// Marshal configuration with JSON.
func Marshal(value interface{}) ([]byte, error) {
	// need to prettyprint, hence MarshalIndent, instead of Encoder
	return json.MarshalIndent(value, "", "  ")
}

func FromMap(v map[string]interface{}) (*Config, error) {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(v); err != nil {
		return nil, err
	}
	var conf Config
	if err := json.NewDecoder(buf).Decode(&conf); err != nil {
		return nil, fmt.Errorf("failure to decode config: %w", err)
	}
	return &conf, nil
}

func ToMap(conf *Config) (map[string]interface{}, error) {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(conf); err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.NewDecoder(buf).Decode(&m); err != nil {
		return nil, fmt.Errorf("failure to decode config: %w", err)
	}
	return m, nil
}

// Convert config to a map, without using encoding/json, since
// zero/empty/'omitempty' fields are excluded by encoding/json during
// marshaling.
func ReflectToMap(conf interface{}) interface{} {
	v := reflect.ValueOf(conf)
	if !v.IsValid() {
		return nil
	}

	// Handle pointer type
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			// Create a zero value of the pointer's element type
			elemType := v.Type().Elem()
			zero := reflect.Zero(elemType)
			return ReflectToMap(zero.Interface())
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		result := make(map[string]interface{})
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			// Only include exported fields
			if field.CanInterface() {
				result[t.Field(i).Name] = ReflectToMap(field.Interface())
			}
		}
		return result

	case reflect.Map:
		result := make(map[string]interface{})
		iter := v.MapRange()
		for iter.Next() {
			key := iter.Key()
			// Convert map keys to strings for consistency
			keyStr := fmt.Sprint(ReflectToMap(key.Interface()))
			result[keyStr] = ReflectToMap(iter.Value().Interface())
		}
		// Add a sample to differentiate between a map and a struct on validation.
		sample := reflect.Zero(v.Type().Elem())
		if sample.CanInterface() {
			result["*"] = ReflectToMap(sample.Interface())
		}
		return result

	case reflect.Slice, reflect.Array:
		result := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = ReflectToMap(v.Index(i).Interface())
		}
		return result

	default:
		// For basic types (int, string, etc.), just return the value
		if v.CanInterface() {
			return v.Interface()
		}
		return nil
	}
}

// Clone copies the config. Use when updating.
func (c *Config) Clone() (*Config, error) {
	var newConfig Config
	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(c); err != nil {
		return nil, fmt.Errorf("failure to encode config: %w", err)
	}

	if err := json.NewDecoder(&buf).Decode(&newConfig); err != nil {
		return nil, fmt.Errorf("failure to decode config: %w", err)
	}

	return &newConfig, nil
}

// Check if the provided key is present in the structure.
func CheckKey(key string) error {
	conf := Config{}

	// Convert an empty config to a map without JSON.
	cursor := ReflectToMap(&conf)

	// Parse the key and verify it's presence in the map.
	var ok bool
	var mapCursor map[string]interface{}

	parts := strings.Split(key, ".")
	for i, part := range parts {
		mapCursor, ok = cursor.(map[string]interface{})
		if !ok {
			if cursor == nil {
				return nil
			}
			path := strings.Join(parts[:i], ".")
			return fmt.Errorf("%s key is not a map", path)
		}

		cursor, ok = mapCursor[part]
		if !ok {
			// If the config sections is a map, validate against the default entry.
			if cursor, ok = mapCursor["*"]; ok {
				continue
			}
			path := strings.Join(parts[:i+1], ".")
			return fmt.Errorf("%s not found", path)
		}
	}
	return nil
}
