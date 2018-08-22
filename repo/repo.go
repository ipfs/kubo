package repo

import (
	"errors"
	"io"

	filestore "github.com/ipfs/go-ipfs/filestore"
	keystore "github.com/ipfs/go-ipfs/keystore"

	config "gx/ipfs/QmTyiSs9VgdVb4pnzdjtKhcfdTkHFEaNn6xnCbZq4DTFRt/go-ipfs-config"
	ds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
)

var (
	ErrApiNotRunning = errors.New("api not running")
)

// Repo represents all persistent data of a given ipfs node.
type Repo interface {
	// Config returns the ipfs configuration file from the repo. Changes made
	// to the returned config are not automatically persisted.
	Config() (*config.Config, error)

	// BackupConfig creates a backup of the current configuration file using
	// the given prefix for naming.
	BackupConfig(prefix string) (string, error)

	// SetConfig persists the given configuration struct to storage.
	SetConfig(*config.Config) error

	// SetConfigKey sets the given key-value pair within the config and persists it to storage.
	SetConfigKey(key string, value interface{}) error

	// GetConfigKey reads the value for the given key from the configuration in storage.
	GetConfigKey(key string) (interface{}, error)

	// Datastore returns a reference to the configured data storage backend.
	Datastore() Datastore

	// GetStorageUsage returns the number of bytes stored.
	GetStorageUsage() (uint64, error)

	// Keystore returns a reference to the key management interface.
	Keystore() keystore.Keystore

	// FileManager returns a reference to the filestore file manager.
	FileManager() *filestore.FileManager

	// SetAPIAddr sets the API address in the repo.
	SetAPIAddr(addr ma.Multiaddr) error

	// SwarmKey returns the configured shared symmetric key for the private networks feature.
	SwarmKey() ([]byte, error)

	io.Closer
}

// Datastore is the interface required from a datastore to be
// acceptable to FSRepo.
type Datastore interface {
	ds.Batching // should be threadsafe, just be careful
	io.Closer
}
