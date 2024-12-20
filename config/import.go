package config

const (
	DefaultCidVersion      = 0
	DefaultUnixFSRawLeaves = false
	DefaultUnixFSChunker   = "size-262144"
	DefaultHashFunction    = "sha2-256"

	// DefaultBatchMaxNodes controls the maximum number of nodes in a
	// write-batch. The total size of the batch is limited by
	// BatchMaxnodes and BatchMaxSize.
	DefaultBatchMaxNodes = 128
	// DefaultBatchMaxSize controls the maximum size of a single
	// write-batch. The total size of the batch is limited by
	// BatchMaxnodes and BatchMaxSize.
	DefaultBatchMaxSize = 100 << 20 // 20MiB
)

// Import configures the default options for ingesting data. This affects commands
// that ingest data, such as 'ipfs add', 'ipfs dag put, 'ipfs block put', 'ipfs files write'.
type Import struct {
	CidVersion      OptionalInteger
	UnixFSRawLeaves Flag
	UnixFSChunker   OptionalString
	HashFunction    OptionalString
	BatchMaxNodes   OptionalInteger
	BatchMaxSize    OptionalInteger
}
