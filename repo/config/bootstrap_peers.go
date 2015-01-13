package config

import (
	"errors"
	"strings"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
)

// BootstrapPeer is a peer used to bootstrap the network.
type BootstrapPeer struct {
	Address string
	PeerID  string // until multiaddr supports ipfs, use another field.
}

func (bp *BootstrapPeer) String() string {
	return bp.Address + "/" + bp.PeerID
}

func ParseBootstrapPeer(addr string) (BootstrapPeer, error) {
	// to be replaced with just multiaddr parsing, once ptp is a multiaddr protocol
	idx := strings.LastIndex(addr, "/")
	if idx == -1 {
		return BootstrapPeer{}, errors.New("invalid address")
	}
	addrS := addr[:idx]
	peeridS := addr[idx+1:]

	// make sure addrS parses as a multiaddr.
	if len(addrS) > 0 {
		maddr, err := ma.NewMultiaddr(addrS)
		if err != nil {
			return BootstrapPeer{}, err
		}

		addrS = maddr.String()
	}

	// make sure idS parses as a peer.ID
	_, err := mh.FromB58String(peeridS)
	if err != nil {
		return BootstrapPeer{}, err
	}

	return BootstrapPeer{
		Address: addrS,
		PeerID:  peeridS,
	}, nil
}

func ParseBootstrapPeers(addrs []string) ([]BootstrapPeer, error) {
	peers := make([]BootstrapPeer, len(addrs))
	var err error
	for i, addr := range addrs {
		peers[i], err = ParseBootstrapPeer(addr)
		if err != nil {
			return nil, err
		}
	}
	return peers, nil
}
