// package config implements the ipfs config file datastructures and utilities.
package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	ic "github.com/jbenet/go-ipfs/p2p/crypto"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("config")

// Identity tracks the configuration of the local node's identity.
type Identity struct {
	PeerID  string
	PrivKey string
}

// Logs tracks the configuration of the event logger
type Logs struct {
	Filename   string
	MaxSizeMB  uint64
	MaxBackups uint64
	MaxAgeDays uint64
}

// Datastore tracks the configuration of the datastore.
type Datastore struct {
	Type string
	Path string
}

// Addresses stores the (string) multiaddr addresses for the node.
type Addresses struct {
	Swarm   []string // addresses for the swarm network
	API     string   // address for the local API (RPC)
	Gateway string   // address to listen on for IPFS HTTP object gateway
}

// Mounts stores the (string) mount points
type Mounts struct {
	IPFS string
	IPNS string
}

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

// Tour stores the ipfs tour read-list and resume point
type Tour struct {
	Last string // last tour topic read
	// Done []string // all topics done so far
}

// Config is used to load IPFS config files.
type Config struct {
	Identity  Identity        // local node's peer identity
	Datastore Datastore       // local node's storage
	Addresses Addresses       // local node's addresses
	Mounts    Mounts          // local node's mount points
	Version   Version         // local node's version management
	Bootstrap []BootstrapPeer // local nodes's bootstrap peers
	Tour      Tour            // local node's tour position
	Logs      Logs            // local node's event log configuration
}

// DefaultPathRoot is the path to the default config dir location.
const DefaultPathRoot = "~/.go-ipfs"

// DefaultConfigFile is the filename of the configuration file
const DefaultConfigFile = "config"

// DefaultDataStoreDirectory is the directory to store all the local IPFS data.
const DefaultDataStoreDirectory = "datastore"

// EnvDir is the environment variable used to change the path root.
const EnvDir = "IPFS_DIR"

// LogsDefaultDirectory is the directory to store all IPFS event logs.
var LogsDefaultDirectory = "logs"

// PathRoot returns the default configuration root directory
func PathRoot() (string, error) {
	dir := os.Getenv(EnvDir)
	var err error
	if len(dir) == 0 {
		dir, err = u.TildeExpansion(DefaultPathRoot)
	}
	return dir, err
}

// Path returns the path `extension` relative to the configuration root. If an
// empty string is provided for `configroot`, the default root is used.
func Path(configroot, extension string) (string, error) {
	if len(configroot) == 0 {
		dir, err := PathRoot()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, extension), nil

	}
	return filepath.Join(configroot, extension), nil
}

// DataStorePath returns the default data store path given a configuration root
// (set an empty string to have the default configuration root)
func DataStorePath(configroot string) (string, error) {
	return Path(configroot, DefaultDataStoreDirectory)
}

// LogsPath returns the default path for event logs given a configuration root
// (set an empty string to have the default configuration root)
func LogsPath(configroot string) (string, error) {
	return Path(configroot, LogsDefaultDirectory)
}

// Filename returns the configuration file path given a configuration root
// directory. If the configuration root directory is empty, use the default one
func Filename(configroot string) (string, error) {
	return Path(configroot, DefaultConfigFile)
}

// DecodePrivateKey is a helper to decode the users PrivateKey
func (i *Identity) DecodePrivateKey(passphrase string) (ic.PrivKey, error) {
	pkb, err := base64.StdEncoding.DecodeString(i.PrivKey)
	if err != nil {
		return nil, err
	}

	// currently storing key unencrypted. in the future we need to encrypt it.
	// TODO(security)
	return ic.UnmarshalPrivateKey(pkb)
}

// HumanOutput gets a config value ready for printing
func HumanOutput(value interface{}) ([]byte, error) {
	s, ok := value.(string)
	if ok {
		return []byte(strings.Trim(s, "\n")), nil
	}
	return Marshal(value)
}

// Marshal configuration with JSON
func Marshal(value interface{}) ([]byte, error) {
	// need to prettyprint, hence MarshalIndent, instead of Encoder
	return json.MarshalIndent(value, "", "  ")
}

func FromMap(v map[string]interface{}) (*Config, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	var conf Config
	if err := json.NewDecoder(&buf).Decode(&conf); err != nil {
		return nil, fmt.Errorf("Failure to decode config: %s", err)
	}
	return &conf, nil
}

func ToMap(conf *Config) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(conf); err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.NewDecoder(&buf).Decode(&m); err != nil {
		return nil, fmt.Errorf("Failure to decode config: %s", err)
	}
	return m, nil
}
