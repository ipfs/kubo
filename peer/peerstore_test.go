package peer

import (
	"testing"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func IDS(t *testing.T, ids string) ID {
	id, err := IDB58Decode(ids)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func MA(t *testing.T, m string) ma.Multiaddr {
	maddr, err := ma.NewMultiaddr(m)
	if err != nil {
		t.Fatal(err)
	}
	return maddr
}

func TestAddresses(t *testing.T) {

	ps := NewPeerstore()

	id1 := IDS(t, "QmcNstKuwBBoVTpSCSDrwzjgrRcaYXK833Psuz2EMHwyQN")
	id2 := IDS(t, "QmRmPL3FDZKE3Qiwv1RosLdwdvbvg17b2hB39QPScgWKKZ")
	id3 := IDS(t, "QmPhi7vBsChP7sjRoZGgg7bcKqF6MmCcQwvRbDte8aJ6Kn")

	ma11 := MA(t, "/ip4/1.2.3.1/tcp/1111")
	ma21 := MA(t, "/ip4/1.2.3.2/tcp/1111")
	ma22 := MA(t, "/ip4/1.2.3.2/tcp/2222")
	ma31 := MA(t, "/ip4/1.2.3.3/tcp/1111")
	ma32 := MA(t, "/ip4/1.2.3.3/tcp/2222")
	ma33 := MA(t, "/ip4/1.2.3.3/tcp/3333")

	ps.AddAddress(id1, ma11)
	ps.AddAddress(id2, ma21)
	ps.AddAddress(id2, ma22)
	ps.AddAddress(id3, ma31)
	ps.AddAddress(id3, ma32)
	ps.AddAddress(id3, ma33)

	a1 := ps.Addresses(id1)
	a2 := ps.Addresses(id2)
	a3 := ps.Addresses(id3)

	test := func(exp, act []ma.Multiaddr) {
		if len(exp) != len(act) {
			t.Fatal("lengths not the same")
		}

		for _, a := range exp {
			found := false

			for _, b := range act {
				if a.Equal(b) {
					found = true
					break
				}
			}

			if !found {
				t.Fatal("expected address %s not found", a)
			}
		}
	}

	test([]ma.Multiaddr{ma11}, a1)
	test([]ma.Multiaddr{ma21, ma22}, a2)
	test([]ma.Multiaddr{ma31, ma32, ma33}, a3)
}
