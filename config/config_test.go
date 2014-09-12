package config

import (
	"testing"
)

func TestConfig(t *testing.T) {
	const filename = ".ipfsconfig"
	const dsPath = "/path/to/datastore"
	cfgWritten := new(Config)
	cfgWritten.Datastore.Path = dsPath
	err := WriteConfigFile(filename, cfgWritten)
	if err != nil {
		t.Error(err)
	}
	cfgRead, err := Load(filename)
	if err != nil {
		t.Error(err)
		return
	}
	if cfgWritten.Datastore.Path != cfgRead.Datastore.Path {
		t.Fail()
	}
}
