package config

import (
	"sort"
	"testing"
)

func TestBoostrapPeerStrings(t *testing.T) {
	parsed, err := ParseBootstrapPeers(DefaultBootstrapAddresses)
	if err != nil {
		t.Fatal(err)
	}

	formatted := BootstrapPeerStrings(parsed)
	sort.Strings(formatted)
	expected := append([]string{}, DefaultBootstrapAddresses...)
	sort.Strings(expected)

	for i, s := range formatted {
		if expected[i] != s {
			t.Fatalf("expected %s, %s", expected[i], s)
		}
	}
}
