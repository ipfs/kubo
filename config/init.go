package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func Init(out io.Writer, nBitsForKeypair int) (*Config, error) {
	identity, err := CreateIdentity(out, []options.KeyGenerateOption{options.Key.Size(nBitsForKeypair)})
	if err != nil {
		return nil, err
	}

	return InitWithIdentity(identity)
}

func InitWithIdentity(identity Identity) (*Config, error) {
	bootstrapPeers, err := DefaultBootstrapPeers()
	if err != nil {
		return nil, err
	}

	datastore := DefaultDatastoreConfig()

	conf := &Config{
		API: API{
			HTTPHeaders: map[string][]string{},
		},

		// setup the node's default addresses.
		// NOTE: two swarm listen addrs, one tcp, one utp.
		Addresses: addressesConfig(),

		Datastore: datastore,
		Bootstrap: BootstrapPeerStrings(bootstrapPeers),
		Identity:  identity,
		Discovery: Discovery{
			MDNS: MDNS{
				Enabled: true,
			},
		},

		Routing: Routing{
			Type:    nil,
			Methods: nil,
			Routers: nil,
		},

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
			NoFetch:      false,
			HTTPHeaders:  map[string][]string{},
		},
		Reprovider: Reprovider{
			Interval: nil,
			Strategy: nil,
		},
		Pinning: Pinning{
			RemoteServices: map[string]RemotePinningService{},
		},
		DNS: DNS{
			Resolvers: map[string]string{},
		},
		Migration: Migration{
			DownloadSources: []string{},
			Keep:            "",
		},
	}

	return conf, nil
}

// DefaultConnMgrHighWater is the default value for the connection managers
// 'high water' mark.
const DefaultConnMgrHighWater = 96

// DefaultConnMgrLowWater is the default value for the connection managers 'low
// water' mark.
const DefaultConnMgrLowWater = 32

// DefaultConnMgrGracePeriod is the default value for the connection managers
// grace period.
const DefaultConnMgrGracePeriod = time.Second * 20

// DefaultConnMgrType is the default value for the connection managers
// type.
const DefaultConnMgrType = "basic"

// DefaultResourceMgrMinInboundConns is a MAGIC number that probably a good
// enough number of inbound conns to be a good network citizen.
const DefaultResourceMgrMinInboundConns = 800

func addressesConfig() Addresses {
	return Addresses{
		Swarm: []string{
			"/ip4/0.0.0.0/tcp/4001",
			"/ip6/::/tcp/4001",
			"/ip4/0.0.0.0/udp/4001/webrtc-direct",
			"/ip4/0.0.0.0/udp/4001/quic-v1",
			"/ip4/0.0.0.0/udp/4001/quic-v1/webtransport",
			"/ip6/::/udp/4001/webrtc-direct",
			"/ip6/::/udp/4001/quic-v1",
			"/ip6/::/udp/4001/quic-v1/webtransport",
		},
		Announce:       []string{},
		AppendAnnounce: []string{},
		NoAnnounce:     []string{},
		API:            Strings{"/ip4/127.0.0.1/tcp/5001"},
		Gateway:        Strings{"/ip4/127.0.0.1/tcp/8080"},
	}
}

// DefaultDatastoreConfig is an internal function exported to aid in testing.
func DefaultDatastoreConfig() Datastore {
	return Datastore{
		StorageMax:         "10GB",
		StorageGCWatermark: 90, // 90%
		GCPeriod:           "1h",
		BloomFilterSize:    0,
		Spec:               flatfsSpec(),
	}
}

func pebbleSpec() map[string]interface{} {
	return map[string]interface{}{
		"type":   "measure",
		"prefix": "pebble.datastore",
		"child": map[string]interface{}{
			"type": "pebbleds",
			"path": "pebbleds",
		},
	}
}

func badgerSpec() map[string]interface{} {
	return map[string]interface{}{
		"type":   "measure",
		"prefix": "badger.datastore",
		"child": map[string]interface{}{
			"type":       "badgerds",
			"path":       "badgerds",
			"syncWrites": false,
			"truncate":   true,
		},
	}
}

func flatfsSpec() map[string]interface{} {
	return map[string]interface{}{
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
	}
}

// CreateIdentity initializes a new identity.
func CreateIdentity(out io.Writer, opts []options.KeyGenerateOption) (Identity, error) {
	// TODO guard higher up
	ident := Identity{}

	settings, err := options.KeyGenerateOptions(opts...)
	if err != nil {
		return ident, err
	}

	var sk crypto.PrivKey
	var pk crypto.PubKey

	switch settings.Algorithm {
	case "rsa":
		if settings.Size == -1 {
			settings.Size = options.DefaultRSALen
		}

		fmt.Fprintf(out, "generating %d-bit RSA keypair...", settings.Size)

		priv, pub, err := crypto.GenerateKeyPair(crypto.RSA, settings.Size)
		if err != nil {
			return ident, err
		}

		sk = priv
		pk = pub
	case "ed25519":
		if settings.Size != -1 {
			return ident, fmt.Errorf("number of key bits does not apply when using ed25519 keys")
		}
		fmt.Fprintf(out, "generating ED25519 keypair...")
		priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return ident, err
		}

		sk = priv
		pk = pub
	default:
		return ident, fmt.Errorf("unrecognized key type: %s", settings.Algorithm)
	}
	fmt.Fprintf(out, "done\n")

	// currently storing key unencrypted. in the future we need to encrypt it.
	// TODO(security)
	skbytes, err := crypto.MarshalPrivateKey(sk)
	if err != nil {
		return ident, err
	}
	ident.PrivKey = base64.StdEncoding.EncodeToString(skbytes)

	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return ident, err
	}
	ident.PeerID = id.String()
	fmt.Fprintf(out, "peer identity: %s\n", ident.PeerID)
	return ident, nil
}
