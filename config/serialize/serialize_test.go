package fsrepo

import (
	"github.com/ipfs/go-ipfs/repo/common"
	"os"
	"runtime"
	"testing"

	config "github.com/ipfs/go-ipfs/config"
)

func TestConfig(t *testing.T) {
	const filename = ".ipfsconfig"
	cfgWritten := new(config.Config)
	cfgWritten.Identity.PeerID = "faketest"

	err := WriteConfigFile(filename, cfgWritten)
	if err != nil {
		t.Fatal(err)
	}
	cfgRead, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	peerId, err := common.MapGetKV(cfgRead, "Identity.PeerID")
	if err != nil {
		t.Fatal(err)
	}

	if cfgWritten.Identity.PeerID != peerId.(string) {
		t.Fatal()
	}
	st, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("cannot stat config file: %v", err)
	}

	if runtime.GOOS != "windows" { // see https://golang.org/src/os/types_windows.go
		if g := st.Mode().Perm(); g&0117 != 0 {
			t.Fatalf("config file should not be executable or accessible to world: %v", g)
		}
	}
}
