package config

// DefaultDataStoreDirectory is the directory to store all the local IPFS data.
const DefaultDataStoreDirectory = "datastore"

// Datastore tracks the configuration of the datastore.
type Datastore struct {
	StorageMax         string // in B, kB, kiB, MB, ...
	StorageGCWatermark int64  // in percentage to multiply on StorageMax
	GCPeriod           string // in ns, us, ms, s, m, h
	Path               string
	NoSync             bool // deprecated

	Spec map[string]interface{}

	HashOnRead      bool
	BloomFilterSize int
}

type S3Datastore struct {
	Region string `json:"region"`
	Bucket string `json:"bucket"`
	ACL    string `json:"acl"`
}

type FlatDS struct {
	Path      string
	ShardFunc string
	Sync      bool
}

type LevelDB struct {
	Path        string
	Compression string
}

type SbsDS struct {
	Path string
}

// DataStorePath returns the default data store path given a configuration root
// (set an empty string to have the default configuration root)
func DataStorePath(configroot string) (string, error) {
	return Path(configroot, DefaultDataStoreDirectory)
}
