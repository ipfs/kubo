package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ReadConfigFile reads the config from `filename` into `cfg`.
func ReadConfigFile(filename string, cfg interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return Decode(f, cfg)
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
	buf, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	_, err = w.Write(buf)
	return err
}

// Decode configuration with JSON
func Decode(r io.Reader, value interface{}) error {
	return json.NewDecoder(r).Decode(value)
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
