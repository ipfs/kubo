package config

import (
	"os"
	"runtime"
	"testing"
)

func TestClone(t *testing.T) {
	c := new(Config)
	c.Identity.PeerID = "faketest"
	c.API.HTTPHeaders = map[string][]string{"foo": {"bar"}}

	newCfg, err := c.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if newCfg.Identity.PeerID != c.Identity.PeerID {
		t.Fatal("peer ID not preserved")
	}

	c.API.HTTPHeaders["foo"] = []string{"baz"}
	if newCfg.API.HTTPHeaders["foo"][0] != "bar" {
		t.Fatal("HTTP headers not preserved")
	}

	delete(c.API.HTTPHeaders, "foo")
	if newCfg.API.HTTPHeaders["foo"][0] != "bar" {
		t.Fatal("HTTP headers not preserved")
	}
}

func TestConfig(t *testing.T) {
	const filename = ".ipfsconfig"
	cfgWritten := new(Config)
	cfgWritten.Identity.PeerID = "faketest"

	err := WriteConfigFile(filename, cfgWritten)
	if err != nil {
		t.Fatal(err)
	}
	cfgRead, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	if cfgWritten.Identity.PeerID != cfgRead.Identity.PeerID {
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

func TestConfigUnknownField(t *testing.T) {
	const filename = ".ipfsconfig"

	badConfig := map[string]string{
		"BadField": "Value",
	}

	err := WriteConfigFile(filename, badConfig)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(filename)
	if err == nil {
		t.Fatal("load must fail")
	}

	if err.Error() != "failure to decode config: json: unknown field \"BadField\"" {
		t.Fatal("unexpected error:", err)
	}

	mapOut := make(map[string]string)

	err = ReadConfigFile(filename, &mapOut)
	if err != nil {
		t.Fatal(err)
	}

	if mapOut["BadField"] != "Value" {
		t.Fatal(err)
	}
}
