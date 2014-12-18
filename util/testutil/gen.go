package testutil

import (
	crand "crypto/rand"
	"fmt"
	"math/rand"

	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func RandPeer() peer.Peer {
	id := make([]byte, 16)
	crand.Read(id)
	mhid := u.Hash(id)
	return NewPeerWithID(peer.ID(mhid))
}

func PeerWithNewKeys() (peer.Peer, error) {
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		return nil, err
	}

	return NewPeerWithKeyPair(sk, pk)
}

func PeerWithKeysAndStringAddress(addr string) (peer.Peer, error) {
	maddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}

	return PeerWithKeysAndAddress(maddr)
}

func PeerWithKeysAndAddress(maddr ma.Multiaddr) (peer.Peer, error) {
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		return nil, err
	}

	p, err := NewPeerWithKeyPair(sk, pk)
	if err != nil {
		return nil, err
	}

	p.AddAddress(maddr)
	return p, nil
}

// RandLocalTCPAddress returns a random multiaddr. it suppresses errors
// for nice composability-- do check the address isn't nil.
func RandLocalTCPAddress() ma.Multiaddr {

	// chances are it will work out, but it **might** fail if the port is in use
	// most ports above 10000 aren't in use by long running processes, so yay.
	// (maybe there should be a range of "loopback" ports that are guaranteed
	// to be open for the process, but naturally can only talk to self.)
	addr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 10000+rand.Intn(50000))
	maddr, _ := ma.NewMultiaddr(addr)
	return maddr
}
