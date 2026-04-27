package libp2p

import (
	"testing"

	ma "github.com/multiformats/go-multiaddr"
)

// makeAddrsFactory must drop empty multiaddrs from the input list.
// A zero-component Multiaddr would otherwise reach the host's signed
// peer record and propagate to peers as "/" when they decode the wire
// bytes.
//
// See https://github.com/libp2p/js-libp2p/issues/3478#issuecomment-4322093929
func TestMakeAddrsFactoryDropsEmptyMultiaddrs(t *testing.T) {
	factory, err := makeAddrsFactory(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	good, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	if err != nil {
		t.Fatal(err)
	}

	in := []ma.Multiaddr{nil, good, {}, good}
	out := factory(in)

	if len(out) != 2 {
		t.Fatalf("expected 2 addrs after factory filter, got %d: %v", len(out), out)
	}
	for i, a := range out {
		if len(a) == 0 {
			t.Fatalf("factory returned an empty multiaddr at index %d", i)
		}
	}
}
