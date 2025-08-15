package config

import (
	"sort"
	"testing"

	"github.com/ipfs/boxo/autoconf"
)

func TestBootstrapPeerStrings(t *testing.T) {
	parsed, err := ParseBootstrapPeers(autoconf.FallbackBootstrapPeers)
	if err != nil {
		t.Fatal(err)
	}

	formatted := BootstrapPeerStrings(parsed)
	sort.Strings(formatted)
	expected := append([]string{}, autoconf.FallbackBootstrapPeers...)
	sort.Strings(expected)

	for i, s := range formatted {
		if expected[i] != s {
			t.Fatalf("expected %s, %s", expected[i], s)
		}
	}
}
