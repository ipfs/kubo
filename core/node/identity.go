package node

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
)

func PeerID(id peer.ID) func() peer.ID {
	return func() peer.ID {
		return id
	}
}

// PrivateKey loads the private key from config
func PrivateKey(sk crypto.PrivKey) func(id peer.ID) (crypto.PrivKey, error) {
	return func(id peer.ID) (crypto.PrivKey, error) {
		id2, err := peer.IDFromPrivateKey(sk)
		if err != nil {
			return nil, err
		}

		if id2 != id {
			return nil, fmt.Errorf("private key in config does not match id: %s != %s", id, id2)
		}
		return sk, nil
	}
}
