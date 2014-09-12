package config

import (
	"fmt"
	"testing"
)

func TestConfig(t *testing.T) {
	const filename = ".ipfsconfig"
	cfgWritten := new(Config)
	err := WriteConfigFile(filename, cfgWritten)
	if err != nil {
		t.Error(err)
	}
	cfgRead, err := Load(filename)
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Printf(cfgRead.Datastore.Path)
}
