package config

import (
	"encoding/json"
)

const (
	// DefaultDataStoreDirectory is the directory to store all the local IPFS data.
	DefaultDataStoreDirectory = "datastore"

	// DefaultBlockKeyCacheSize is the size for the blockstore two-queue
	// cache which caches block keys and sizes.
	DefaultBlockKeyCacheSize = 64 << 10

	// DefaultWriteThrough specifies whether to use a "write-through"
	// Blockstore and Blockservice. This means that they will write
	// without performing any reads to check if the incoming blocks are
	// already present in the datastore. Enable for datastores with fast
	// writes and slower reads.
	DefaultWriteThrough bool = true
)

// Datastore tracks the configuration of the datastore.
type Datastore struct {
	StorageMax         string // in B, kB, kiB, MB, ...
	StorageGCWatermark int64  // in percentage to multiply on StorageMax
	GCPeriod           string // in ns, us, ms, s, m, h

	// deprecated fields, use Spec
	Type   string           `json:",omitempty"`
	Path   string           `json:",omitempty"`
	NoSync bool             `json:",omitempty"`
	Params *json.RawMessage `json:",omitempty"`

	Spec map[string]interface{}

	HashOnRead        bool
	BloomFilterSize   int
	BlockKeyCacheSize OptionalInteger `json:",omitempty"`
	WriteThrough      Flag            `json:",omitempty"`
}

// DataStorePath returns the default data store path given a configuration root
// (set an empty string to have the default configuration root).
func DataStorePath(configroot string) (string, error) {
	return Path(configroot, DefaultDataStoreDirectory)
}
