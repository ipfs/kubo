package config

import (
	"encoding/base64"
	"fmt"

	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

func Init(nBitsForKeypair int) (*Config, error) {
	ds, err := datastoreConfig()
	if err != nil {
		return nil, err
	}

	identity, err := identityConfig(nBitsForKeypair)
	if err != nil {
		return nil, err
	}

	bootstrapPeers, err := DefaultBootstrapPeers()
	if err != nil {
		return nil, err
	}

	conf := &Config{

		// setup the node's default addresses.
		// Note: two swarm listen addrs, one tcp, one utp.
		Addresses: Addresses{
			Swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				// "/ip4/0.0.0.0/udp/4002/utp", // disabled for now.
			},
			API: "/ip4/127.0.0.1/tcp/5001",
		},

		Bootstrap: bootstrapPeers,
		Datastore: *ds,
		Identity:  identity,

		// setup the node mount points.
		Mounts: Mounts{
			IPFS: "/ipfs",
			IPNS: "/ipns",
		},

		// tracking ipfs version used to generate the init folder and adding
		// update checker default setting.
		Version: VersionDefaultValue(),
	}

	return conf, nil
}

func datastoreConfig() (*Datastore, error) {
	dspath, err := DataStorePath("")
	if err != nil {
		return nil, err
	}
	return &Datastore{
		Path: dspath,
		Type: "leveldb",
	}, nil
}

// identityConfig initializes a new identity.
func identityConfig(nbits int) (Identity, error) {
	// TODO guard higher up
	ident := Identity{}
	if nbits < 1024 {
		return ident, errors.New("Bitsize less than 1024 is considered unsafe.")
	}

	fmt.Printf("generating %v-bit RSA keypair...", nbits)
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, nbits)
	if err != nil {
		return ident, err
	}
	fmt.Printf("done\n")

	// currently storing key unencrypted. in the future we need to encrypt it.
	// TODO(security)
	skbytes, err := sk.Bytes()
	if err != nil {
		return ident, err
	}
	ident.PrivKey = base64.StdEncoding.EncodeToString(skbytes)

	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return ident, err
	}
	ident.PeerID = id.Pretty()
	fmt.Printf("peer identity: %s\n", ident.PeerID)
	return ident, nil
}
