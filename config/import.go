package config

const (
	DefaultCidVersion      = 0
	DefaultUnixFSRawLeaves = false
	DefaultUnixFSChunker   = "size-262144"
	DefaultHashFunction    = "sha2-256"
)

// Import configures the default options for ingesting data. This affects commands
// that ingest data, such as 'ipfs add', 'ipfs dag put, 'ipfs block put', 'ipfs files write'.
type Import struct {
	CidVersion      OptionalInteger
	UnixFSRawLeaves Flag
	UnixFSChunker   OptionalString
	HashFunction    OptionalString
}
