package config

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"os"

	u "github.com/jbenet/go-ipfs/util"
)

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

// BootstrapPeer is a peer used to bootstrap the network.
type BootstrapPeer struct {
	Address string
	PeerID  string // until multiaddr supports ipfs, use another field.
}

// Config is used to load IPFS config files.
type Config struct {
	Identity  Identity         // local node's peer identity
	Datastore Datastore        // local node's storage
	Addresses Addresses        // local node's addresses
	Bootstrap []*BootstrapPeer // local nodes's bootstrap peers
}

// DefaultPathRoot is the default parth for the IPFS node's root dir.
const DefaultPathRoot = "~/.go-ipfs"

// DefaultConfigFilePath points to the ipfs node config file.
const DefaultConfigFilePath = DefaultPathRoot + "/config"

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

// Filename returns the proper tilde expanded config filename.
func Filename(filename string) (string, error) {
	if len(filename) == 0 {
		filename = DefaultConfigFilePath
	}

	// tilde expansion on config file
	return u.TildeExpansion(filename)
}

// Load reads given file and returns the read config, or error.
func Load(filename string) (*Config, error) {
	filename, err := Filename(filename)
	if err != nil {
		return nil, err
	}

	// if nothing is there, fail. User must run 'ipfs init'
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, errors.New("ipfs not initialized, please run 'ipfs init'")
	}

	var cfg Config
	err = ReadConfigFile(filename, &cfg)
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
