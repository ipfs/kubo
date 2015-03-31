package fsrepo

import (
	"os"
	"testing"

	config "github.com/ipfs/go-ipfs/repo/config"
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
	st, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("cannot stat config file: %v", err)
	}
	if g := st.Mode().Perm(); g&0117 != 0 {
		t.Errorf("config file should not be executable or accessible to world: %v", g)
	}
}
