package config

import (
	"encoding/json"
	"errors"
	"fmt"
	u "github.com/jbenet/go-ipfs/util"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// WriteFile writes the given buffer `buf` into file named `filename`.
func WriteFile(filename string, buf []byte) error {
	err := os.MkdirAll(path.Dir(filename), 0777)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, buf, 0666)
}

// ReadConfigFile reads the config from `filename` into `cfg`.
func ReadConfigFile(filename string, cfg *Config) error {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(buf, cfg)
}

// WriteConfigFile writes the config from `cfg` into `filename`.
func WriteConfigFile(filename string, cfg *Config) error {
	buf, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return WriteFile(filename, buf)
}

// WriteConfigFile writes the config from `cfg` into `filename`.
func GetValueInConfigFile(key string) (value string, err error) {
	// reading config file
	attrs := strings.Split(key, ".")

	filename, _ := u.TildeExpansion(defaultConfigFilePath)
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	// deserializing json
	var cfg interface{}
	var exists bool

	err = json.Unmarshal(buf, &cfg)
	if err != nil {
		return "", err
	}

	for i := range attrs {
		cfgMap, isMap := cfg.(map[string]interface{})
		if !isMap {
			return "", errors.New(fmt.Sprintf("%s has no attributes", strings.Join(attrs[:i], ".")))
		}
		cfg, exists = cfgMap[attrs[i]]
		if !exists {
			return "", errors.New(fmt.Sprintf("Configuration option key \"%s\" not recognized", strings.Join(attrs[:i+1], ".")))
		}
		val, is_string := cfg.(string)
		if is_string {
			return val, nil
		}
	}
	return "", errors.New(fmt.Sprintf("%s is not a string", key))
}

// WriteConfigFile writes the config from `cfg` into `filename`.
func SetValueInConfigFile(key string, values []string) error {
	assignee := strings.Join(values, " ")
	attrs := strings.Split(key, ".")

	filename, _ := u.TildeExpansion(defaultConfigFilePath)
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	// deserializing json
	var cfg, orig interface{}
	var exists, isMap bool
	cfgMap := make(map[string]interface{})

	err = json.Unmarshal(buf, &orig)
	cfg = orig
	if err != nil {
		return err
	}

	for i := 0; i < len(attrs); i++ {
		cfgMap, isMap = cfg.(map[string]interface{})
		// curs = append(curs, cfgMap)
		if !isMap {
			return errors.New(fmt.Sprintf("%s has no attributes", strings.Join(attrs[:i], ".")))
		}
		cfg, exists = cfgMap[attrs[i]]
		if !exists {
			return errors.New(fmt.Sprintf("Configuration option key \"%s\" not recognized", strings.Join(attrs[:i+1], ".")))
		}
	}
	cfgMap[attrs[len(attrs)-1]] = assignee
	buf, err = json.MarshalIndent(orig, "", "  ")
	if err != nil {
		return err
	}
	WriteFile(filename, buf)
	return nil
}
