package peer

import (
	"testing"
	"time"

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
	id4 := IDS(t, "QmPhi7vBsChP7sjRoZGgg7bcKqF6MmCcQwvRbDte8aJ5Kn")
	id5 := IDS(t, "QmPhi7vBsChP7sjRoZGgg7bcKqF6MmCcQwvRbDte8aJ5Km")

	ma11 := MA(t, "/ip4/1.2.3.1/tcp/1111")
	ma21 := MA(t, "/ip4/2.2.3.2/tcp/1111")
	ma22 := MA(t, "/ip4/2.2.3.2/tcp/2222")
	ma31 := MA(t, "/ip4/3.2.3.3/tcp/1111")
	ma32 := MA(t, "/ip4/3.2.3.3/tcp/2222")
	ma33 := MA(t, "/ip4/3.2.3.3/tcp/3333")
	ma41 := MA(t, "/ip4/4.2.3.3/tcp/1111")
	ma42 := MA(t, "/ip4/4.2.3.3/tcp/2222")
	ma43 := MA(t, "/ip4/4.2.3.3/tcp/3333")
	ma44 := MA(t, "/ip4/4.2.3.3/tcp/4444")
	ma51 := MA(t, "/ip4/5.2.3.3/tcp/1111")
	ma52 := MA(t, "/ip4/5.2.3.3/tcp/2222")
	ma53 := MA(t, "/ip4/5.2.3.3/tcp/3333")
	ma54 := MA(t, "/ip4/5.2.3.3/tcp/4444")
	ma55 := MA(t, "/ip4/5.2.3.3/tcp/5555")

	ps.AddAddress(id1, ma11)
	ps.AddAddresses(id2, []ma.Multiaddr{ma21, ma22})
	ps.AddAddresses(id2, []ma.Multiaddr{ma21, ma22}) // idempotency
	ps.AddAddress(id3, ma31)
	ps.AddAddress(id3, ma32)
	ps.AddAddress(id3, ma33)
	ps.AddAddress(id3, ma33) // idempotency
	ps.AddAddress(id3, ma33)
	ps.AddAddresses(id4, []ma.Multiaddr{ma41, ma42, ma43, ma44})       // multiple
	ps.AddAddresses(id5, []ma.Multiaddr{ma21, ma22})                   // clearing
	ps.AddAddresses(id5, []ma.Multiaddr{ma41, ma42, ma43, ma44})       // clearing
	ps.SetAddresses(id5, []ma.Multiaddr{ma51, ma52, ma53, ma54, ma55}) // clearing

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

	// test the Addresses return value
	test([]ma.Multiaddr{ma11}, ps.Addresses(id1))
	test([]ma.Multiaddr{ma21, ma22}, ps.Addresses(id2))
	test([]ma.Multiaddr{ma31, ma32, ma33}, ps.Addresses(id3))
	test([]ma.Multiaddr{ma41, ma42, ma43, ma44}, ps.Addresses(id4))
	test([]ma.Multiaddr{ma51, ma52, ma53, ma54, ma55}, ps.Addresses(id5))

	// test also the PeerInfo return
	test([]ma.Multiaddr{ma11}, ps.PeerInfo(id1).Addrs)
	test([]ma.Multiaddr{ma21, ma22}, ps.PeerInfo(id2).Addrs)
	test([]ma.Multiaddr{ma31, ma32, ma33}, ps.PeerInfo(id3).Addrs)
	test([]ma.Multiaddr{ma41, ma42, ma43, ma44}, ps.PeerInfo(id4).Addrs)
	test([]ma.Multiaddr{ma51, ma52, ma53, ma54, ma55}, ps.PeerInfo(id5).Addrs)
}

func TestAddressTTL(t *testing.T) {

	ps := NewPeerstore()
	id1 := IDS(t, "QmcNstKuwBBoVTpSCSDrwzjgrRcaYXK833Psuz2EMHwyQN")
	ma1 := MA(t, "/ip4/1.2.3.1/tcp/1111")
	ma2 := MA(t, "/ip4/2.2.3.2/tcp/2222")
	ma3 := MA(t, "/ip4/3.2.3.3/tcp/3333")
	ma4 := MA(t, "/ip4/4.2.3.3/tcp/4444")
	ma5 := MA(t, "/ip4/5.2.3.3/tcp/5555")

	ps.AddAddress(id1, ma1)
	ps.AddAddress(id1, ma2)
	ps.AddAddress(id1, ma3)
	ps.AddAddress(id1, ma4)
	ps.AddAddress(id1, ma5)

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

	testTTL := func(ttle time.Duration, id ID, addr ma.Multiaddr) {
		ab := ps.(*peerstore).addressbook
		ttlat := ab.addrs[id][addr.String()].TTL
		ttla := ttlat.Sub(time.Now())
		if ttla > ttle {
			t.Error("ttl is greater than expected", ttle, ttla)
		}
		if ttla < (ttle / 2) {
			t.Error("ttl is smaller than expected", ttle/2, ttla)
		}
	}

	// should they are there
	ab := ps.(*peerstore).addressbook
	if len(ab.addrs[id1]) != 5 {
		t.Error("incorrect addr count", len(ab.addrs[id1]), ab.addrs[id1])
	}

	// test the Addresses return value
	test([]ma.Multiaddr{ma1, ma2, ma3, ma4, ma5}, ps.Addresses(id1))
	test([]ma.Multiaddr{ma1, ma2, ma3, ma4, ma5}, ps.PeerInfo(id1).Addrs)

	// check the addr TTL is a bit smaller than the init TTL
	testTTL(AddressTTL, id1, ma1)
	testTTL(AddressTTL, id1, ma2)
	testTTL(AddressTTL, id1, ma3)
	testTTL(AddressTTL, id1, ma4)
	testTTL(AddressTTL, id1, ma5)

	// change the TTL
	setTTL := func(id ID, addr ma.Multiaddr, ttl time.Time) {
		a := ab.addrs[id][addr.String()]
		a.TTL = ttl
		ab.addrs[id][addr.String()] = a
	}
	setTTL(id1, ma1, time.Now().Add(-1*time.Second))
	setTTL(id1, ma2, time.Now().Add(-1*time.Hour))
	setTTL(id1, ma3, time.Now().Add(-1*AddressTTL))

	// should no longer list those
	test([]ma.Multiaddr{ma4, ma5}, ps.Addresses(id1))
	test([]ma.Multiaddr{ma4, ma5}, ps.PeerInfo(id1).Addrs)

	// should no longer be there
	if len(ab.addrs[id1]) != 2 {
		t.Error("incorrect addr count", len(ab.addrs[id1]), ab.addrs[id1])
	}
}
