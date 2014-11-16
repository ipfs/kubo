// package config implements the ipfs config file datastructures and utilities.
package config

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"os"
	"path/filepath"

	u "github.com/jbenet/go-ipfs/util"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

var log = u.Logger("config")

// Identity tracks the configuration of the local node's identity.
type Identity struct {
	PeerID  string
	PrivKey string
}

// Datastore tracks the configuration of the datastore.
type Datastore struct {
	Type string
	Path string
}

// Addresses stores the (string) multiaddr addresses for the node.
type Addresses struct {
	Swarm string // address for the swarm network
	API   string // address for the local API (RPC)
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

// Tour stores the ipfs tour read-list and resume point
type Tour struct {
	Last string // last tour topic read
	// Done []string // all topics done so far
}

// Config is used to load IPFS config files.
type Config struct {
	Identity  Identity         // local node's peer identity
	Datastore Datastore        // local node's storage
	Addresses Addresses        // local node's addresses
	Mounts    Mounts           // local node's mount points
	Version   Version          // local node's version management
	Bootstrap []*BootstrapPeer // local nodes's bootstrap peers
	Tour      Tour             // local node's tour position
}

// DefaultPathRoot is the path to the default config dir location.
const DefaultPathRoot = "~/.go-ipfs"

// DefaultConfigFile is the filename of the configuration file
const DefaultConfigFile = "config"

// DefaultDataStoreDirectory is the directory to store all the local IPFS data.
const DefaultDataStoreDirectory = "datastore"

// EnvDir is the environment variable used to change the path root.
const EnvDir = "IPFS_DIR"

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

// Filename returns the configuration file path given a configuration root
// directory. If the configuration root directory is empty, use the default one
func Filename(configroot string) (string, error) {
	return Path(configroot, DefaultConfigFile)
}

// DecodePrivateKey is a helper to decode the users PrivateKey
func (i *Identity) DecodePrivateKey(passphrase string) (crypto.PrivateKey, error) {
	pkb, err := base64.StdEncoding.DecodeString(i.PrivKey)
	if err != nil {
		return nil, err
	}

	// currently storing key unencrypted. in the future we need to encrypt it.
	// TODO(security)
	return x509.ParsePKCS1PrivateKey(pkb)
}

// Load reads given file and returns the read config, or error.
func Load(filename string) (*Config, error) {
	// if nothing is there, fail. User must run 'ipfs init'
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, errors.New("ipfs not initialized, please run 'ipfs init'")
	}

	var cfg Config
	err := ReadConfigFile(filename, &cfg)
	if err != nil {
		return nil, err
	}

	// tilde expansion on datastore path
	cfg.Datastore.Path, err = u.TildeExpansion(cfg.Datastore.Path)
	if err != nil {
		return nil, err
	}

	return &cfg, err
}

// Set sets the value of a particular config key
func Set(filename, key, value string) error {
	return WriteConfigKey(filename, key, value)
}
