package repo

import (
	"context"
	"errors"
	"io"

	filestore "github.com/ipfs/go-filestore"
	keystore "github.com/ipfs/go-ipfs-keystore"

	ds "github.com/ipfs/go-datastore"
	config "github.com/ipfs/go-ipfs/config"
	ma "github.com/multiformats/go-multiaddr"
)

var (
	ErrApiNotRunning = errors.New("api not running")
)

// FIXME: Anything returning config.Config here would now be returning
//  the system config: internal system defaults with the UserConfigOverrides
//  applied over them.
// Repo represents all persistent data of a given ipfs node.
type Repo interface {
	// Config returns the running ipfs configuration from the system defaults
	// overridden where applicable by a user-defined JSON file in the repo.
	// Changes made to the returned config are not automatically persisted, but
	// do impact on the running node.
	// FIXME: Deprecate this in favor of GetSystemConfigKey to have a read-only
	//  configuration that is modified explicitly in SetSystemConfigKey.
	Config() (*config.Config, error)

	// BackupConfig creates a backup of the current configuration file using
	// the given prefix for naming.
	BackupConfig(prefix string) (string, error)

	// SetConfig persists the given configuration struct to storage.
	// FIXME: Deprecate this in favor of `SetConfigKey` to clearly
	//  expose which configuration options is being changed in the API call.
	SetConfig(*config.Config) error

	// SetConfigKey sets the given key-value pair within the system config and
	// also persists it to the user configuration overrides file.
	SetConfigKey(key string, value interface{}) error

	// GetConfigKey reads the value for the given key from the configuration in storage.
	// FIXME: Deprecate this and replace it with two distinct APIs:
	//  * GetSystemConfigKey: reads from the running system configuration.
	//  * GetUserConfigOverrideKey: reads from the user override file (`.ipfs/conf`).
	GetConfigKey(key string) (interface{}, error)

	// Datastore returns a reference to the configured data storage backend.
	Datastore() Datastore

	// GetStorageUsage returns the number of bytes stored.
	GetStorageUsage(context.Context) (uint64, error)

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
	ds.Batching // must be thread-safe
}
