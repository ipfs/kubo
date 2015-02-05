package identify

import (
	"testing"
	"time"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// TestObsAddrSet
func TestObsAddrSet(t *testing.T) {
	m := func(s string) ma.Multiaddr {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			t.Error(err)
		}
		return m
	}

	addrsMarch := func(a, b []ma.Multiaddr) bool {
		for _, aa := range a {
			found := false
			for _, bb := range b {
				if aa.Equal(bb) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}

	a1 := m("/ip4/1.2.3.4/tcp/1231")
	a2 := m("/ip4/1.2.3.4/tcp/1232")
	a3 := m("/ip4/1.2.3.4/tcp/1233")

	oas := ObservedAddrSet{}

	if !addrsMarch(oas.Addrs(), nil) {
		t.Error("addrs should be empty")
	}

	oas.Add(a1)
	oas.Add(a2)
	oas.Add(a3)

	// these are all different so we should not yet get them.
	if !addrsMarch(oas.Addrs(), nil) {
		t.Error("addrs should _still_ be empty (once)")
	}

	oas.Add(a1)
	if !addrsMarch(oas.Addrs(), []ma.Multiaddr{a1}) {
		t.Error("addrs should only have a1")
	}

	oas.Add(a2)
	oas.Add(a1)
	oas.Add(a1)
	if !addrsMarch(oas.Addrs(), []ma.Multiaddr{a1, a2}) {
		t.Error("addrs should only have a1, a2")
	}

	// change the timeout constant so we can time it out.
	oas.SetTTL(time.Millisecond * 200)
	<-time.After(time.Millisecond * 210)
	if !addrsMarch(oas.Addrs(), []ma.Multiaddr{nil}) {
		t.Error("addrs should have timed out")
	}
}
