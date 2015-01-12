package fsrepo

import (
	"testing"

	config "github.com/jbenet/go-ipfs/repo/config"
)

func TestConfig(t *testing.T) {
	const filename = ".ipfsconfig"
	const dsPath = "/path/to/datastore"
	cfgWritten := new(config.Config)
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
