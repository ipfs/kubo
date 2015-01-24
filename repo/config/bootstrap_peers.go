package config

import (
	"strings"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

// DefaultBootstrapAddresses are the hardcoded bootstrap addresses
// for ipfs. they are nodes run by the ipfs team. docs on these later.
// As with all p2p networks, bootstrap is an important security concern.
//
// Note: this is here -- and not inside cmd/ipfs/init.go -- because of an
// import dependency issue. TODO: move this into a config/default/ package.
var DefaultBootstrapAddresses = []string{
	"/ip4/104.131.131.82/tcp/4001/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",  // mars.i.ipfs.io
	"/ip4/104.236.176.52/tcp/4001/QmSoLnSGccFuZQJzRadHn95W2CrSFmZuTdDWP8HXaHca9z",  // neptune (to be neptune.i.ipfs.io)
	"/ip4/104.236.179.241/tcp/4001/QmSoLpPVmHKQ4XTPdz8tjDFgdeRFkpV8JgYq8JVJ69RrZm", // pluto (to be pluto.i.ipfs.io)
	"/ip4/162.243.248.213/tcp/4001/QmSoLueR4xBeUbY9WZ9xGUUxunbKWcrNFTDAadQJmocnWm", // uranus (to be uranus.i.ipfs.io)
	"/ip4/128.199.219.111/tcp/4001/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu", // saturn (to be saturn.i.ipfs.io)
	"/ip4/104.236.76.40/tcp/4001/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64",   // venus (to be venus.i.ipfs.io)
	"/ip4/178.62.158.247/tcp/4001/QmSoLer265NRgSp2LA3dPaeykiS1J6DifTC88f5uVQKNAd",  // earth (to be earth.i.ipfs.io)
	"/ip4/178.62.61.185/tcp/4001/QmSoLMeWqB7YGVLJN3pNLQpmmEk35v6wYtsMGLzSr5QBU3",   // mercury (to be mercury.i.ipfs.io)
	"/ip4/104.236.151.122/tcp/4001/QmSoLju6m7xTh3DuokvT3886QRYqxAzb1kShaanJgW36yx", // jupiter (to be jupiter.i.ipfs.io)
}

// BootstrapPeer is a peer used to bootstrap the network.
type BootstrapPeer struct {
	Address string
	PeerID  string // until multiaddr supports ipfs, use another field.
}

// DefaultBootstrapPeers returns the (parsed) set of default bootstrap peers.
// if it fails, it returns a meaningful error for the user.
// This is here (and not inside cmd/ipfs/init) because of module dependency problems.
func DefaultBootstrapPeers() ([]BootstrapPeer, error) {
	ps, err := ParseBootstrapPeers(DefaultBootstrapAddresses)
	if err != nil {
		return nil, errors.Errorf(`failed to parse hardcoded bootstrap peers: %s
This is a problem with the ipfs codebase. Please report it to the dev team.`, err)
	}
	return ps, nil
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
