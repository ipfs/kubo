package config

import (
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
