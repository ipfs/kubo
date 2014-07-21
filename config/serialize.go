package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
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
