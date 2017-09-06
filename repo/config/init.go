package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

func Init(out io.Writer, nBitsForKeypair, keyType int) (*Config, error) {
	identity, err := identityConfig(out, nBitsForKeypair, keyType)
	if err != nil {
		return nil, err
	}

	bootstrapPeers, err := DefaultBootstrapPeers()
	if err != nil {
		return nil, err
	}

	datastore := DefaultDatastoreConfig()

	conf := &Config{

		// setup the node's default addresses.
		// NOTE: two swarm listen addrs, one tcp, one utp.
		Addresses: Addresses{
			Swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				// "/ip4/0.0.0.0/udp/4002/utp", // disabled for now.
				"/ip6/::/tcp/4001",
			},
			Announce:   []string{},
			NoAnnounce: []string{},
			API:        "/ip4/127.0.0.1/tcp/5001",
			Gateway:    "/ip4/127.0.0.1/tcp/8080",
		},

		Datastore: datastore,
		Bootstrap: BootstrapPeerStrings(bootstrapPeers),
		Identity:  identity,
		Discovery: Discovery{MDNS{
			Enabled:  true,
			Interval: 10,
		}},

		// setup the node mount points.
		Mounts: Mounts{
			IPFS: "/ipfs",
			IPNS: "/ipns",
		},

		Ipns: Ipns{
			ResolveCacheSize: 128,
		},

		Gateway: Gateway{
			RootRedirect: "",
			Writable:     false,
			PathPrefixes: []string{},
			HTTPHeaders: map[string][]string{
				"Access-Control-Allow-Origin":  []string{"*"},
				"Access-Control-Allow-Methods": []string{"GET"},
				"Access-Control-Allow-Headers": []string{"X-Requested-With", "Range"},
			},
		},
		Reprovider: Reprovider{
			Interval: "12h",
			Strategy: "all",
		},
	}

	return conf, nil
}

// DefaultDatastoreConfig is an internal function exported to aid in testing.
func DefaultDatastoreConfig() Datastore {
	return Datastore{
		StorageMax:         "10GB",
		StorageGCWatermark: 90, // 90%
		GCPeriod:           "1h",
		BloomFilterSize:    0,
		Spec: map[string]interface{}{
			"type": "mount",
			"mounts": []interface{}{
				map[string]interface{}{
					"mountpoint": "/blocks",
					"type":       "measure",
					"prefix":     "flatfs.datastore",
					"child": map[string]interface{}{
						"type":      "flatfs",
						"path":      "blocks",
						"sync":      true,
						"shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
					},
				},
				map[string]interface{}{
					"mountpoint": "/",
					"type":       "measure",
					"prefix":     "leveldb.datastore",
					"child": map[string]interface{}{
						"type":        "levelds",
						"path":        "datastore",
						"compression": "none",
					},
				},
			},
		},
	}
}

// identityConfig initializes a new identity.
func identityConfig(out io.Writer, nbits, keyType int) (Identity, error) {
	// TODO guard higher up
	ident := Identity{}

	switch keyType {
	case ci.RSA:
		if nbits < 1024 {
			return ident, errors.New("Bitsize less than 1024 is considered unsafe for RSA.")
		}

		fmt.Fprintf(out, "generating %v-bit RSA keypair...", nbits)
	case ci.Ed25519:
		fmt.Fprintf(out, "generating Ed25519 keypair...")
	default:
		return ident, fmt.Errorf("unrecognized keyType: %d", keyType)
	}

	sk, pk, err := ci.GenerateKeyPair(keyType, nbits)
	if err != nil {
		return ident, err
	}
	fmt.Fprintf(out, "done\n")

	// currently storing key unencrypted. in the future we need to encrypt it.
	// TODO(security)
	skbytes, err := sk.Bytes()
	if err != nil {
		return ident, err
	}
	ident.PrivKey = base64.StdEncoding.EncodeToString(skbytes)

	kf := peer.IDFromPublicKey
	switch keyType {
	case ci.RSA:
		kf = peer.IDFromPublicKey
	case ci.Ed25519:
		kf = peer.IDFromEd25519PublicKey
	default:
		return ident, fmt.Errorf("unrecognized keyType: %d", keyType)
	}

	id, err := kf(pk)
	if err != nil {
		return ident, err
	}
	ident.PeerID = id.Pretty()

	fmt.Fprintf(out, "peer identity: %s\n", ident.PeerID)
	return ident, nil
}
