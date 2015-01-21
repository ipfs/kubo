package ipfsaddr

import (
	"errors"
	"strings"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	peer "github.com/jbenet/go-ipfs/p2p/peer"
)

// ErrInvalidAddr signals an address is not a valid ipfs address.
var ErrInvalidAddr = errors.New("invalid ipfs address")

type IPFSAddr interface {
	ID() peer.ID
	Multiaddr() ma.Multiaddr
	String() string
	Equal(b interface{}) bool
}

type ipfsAddr struct {
	ma ma.Multiaddr
	id peer.ID
}

func (a ipfsAddr) ID() peer.ID {
	return a.id
}

func (a ipfsAddr) Multiaddr() ma.Multiaddr {
	return a.ma
}

func (a ipfsAddr) String() string {
	return a.ma.String()
}

func (a ipfsAddr) Equal(b interface{}) bool {
	if ib, ok := b.(IPFSAddr); ok {
		return a.Multiaddr().Equal(ib.Multiaddr())
	}
	if mb, ok := b.(ma.Multiaddr); ok {
		return a.Multiaddr().Equal(mb)
	}
	return false
}

// ParseString parses a string representation of an address into an IPFSAddr
func ParseString(str string) (a IPFSAddr, err error) {
	if str == "" {
		return nil, ErrInvalidAddr
	}

	m, err := ma.NewMultiaddr(str)
	if err != nil {
		return nil, err
	}

	return ParseMultiaddr(m)
}

// ParseMultiaddr parses a multiaddr into an IPFSAddr
func ParseMultiaddr(m ma.Multiaddr) (a IPFSAddr, err error) {
	// // never panic.
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		a = nil
	// 		err = ErrInvalidAddr
	// 	}
	// }()

	if m == nil {
		return nil, ErrInvalidAddr
	}

	// make sure it's an ipfs addr
	parts := ma.Split(m)
	if len(parts) < 1 {
		return nil, ErrInvalidAddr
	}
	ipfspart := parts[len(parts)-1] // last part
	if ipfspart.Protocols()[0].Code != ma.P_IPFS {
		return nil, ErrInvalidAddr
	}

	// make sure ipfs id parses as a peer.ID
	peerIdParts := strings.Split(ipfspart.String(), "/")
	peerIdStr := peerIdParts[len(peerIdParts)-1]
	id, err := peer.IDB58Decode(peerIdStr)
	if err != nil {
		return nil, err
	}

	return ipfsAddr{ma: m, id: id}, nil
}
