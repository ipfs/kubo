package config

import (
	"sort"
	"testing"

	"github.com/ipfs/kubo/boxo/autoconfig"
)

func TestBoostrapPeerStrings(t *testing.T) {
	parsed, err := ParseBootstrapPeers(autoconfig.FallbackBootstrapPeers)
	if err != nil {
		t.Fatal(err)
	}

	formatted := BootstrapPeerStrings(parsed)
	sort.Strings(formatted)
	expected := append([]string{}, autoconfig.FallbackBootstrapPeers...)
	sort.Strings(expected)

	for i, s := range formatted {
		if expected[i] != s {
			t.Fatalf("expected %s, %s", expected[i], s)
		}
	}
}
