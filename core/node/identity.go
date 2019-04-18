package node

import (
	"errors"
	"fmt"

	"github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/libp2p/go-libp2p-peer"
)

func PeerID(cfg *config.Config) (peer.ID, error) {
	cid := cfg.Identity.PeerID
	if cid == "" {
		return "", errors.New("identity was not set in config (was 'ipfs init' run?)")
	}
	if len(cid) == 0 {
		return "", errors.New("no peer ID in config! (was 'ipfs init' run?)")
	}

	id, err := peer.IDB58Decode(cid)
	if err != nil {
		return "", fmt.Errorf("peer ID invalid: %s", err)
	}

	return id, nil
}

func PrivateKey(cfg *config.Config, id peer.ID) (crypto.PrivKey, error) {
	if cfg.Identity.PrivKey == "" {
		return nil, nil
	}

	sk, err := cfg.Identity.DecodePrivateKey("passphrase todo!")
	if err != nil {
		return nil, err
	}

	id2, err := peer.IDFromPrivateKey(sk)
	if err != nil {
		return nil, err
	}

	if id2 != id {
		return nil, fmt.Errorf("private key in config does not match id: %s != %s", id, id2)
	}
	return sk, nil
}
