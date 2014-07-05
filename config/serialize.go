package config

import (
  "os"
  "path"
  "io/ioutil"
  "encoding/json"
)

func ReadFile(filename string) ([]byte, error) {
  return ioutil.ReadFile(filename)
}

func WriteFile(filename string, buf []byte) error {
  err := os.MkdirAll(path.Dir(filename), 0777)
  if err != nil {
    return err
  }

  return ioutil.WriteFile(filename, buf, 0666)
}

func ReadConfigFile(filename string, cfg *Config) error {
  buf, err := ReadFile(filename)
  if err != nil {
    return err
  }

  return json.Unmarshal(buf, cfg)
}

func WriteConfigFile(filename string, cfg *Config) error {
  buf, err := json.MarshalIndent(cfg, "", "  ")
  if err != nil {
    return err
  }

  return WriteFile(filename, buf)
}
