package config

import (
	"github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	"github.com/ipfs/boxo/ipld/unixfs/io"
)

const (
	DefaultCidVersion      = 0
	DefaultUnixFSRawLeaves = false
	DefaultUnixFSChunker   = "size-262144"
	DefaultHashFunction    = "sha2-256"

	DefaultUnixFSHAMTDirectorySizeThreshold = "256KiB" // https://github.com/ipfs/boxo/blob/6c5a07602aed248acc86598f30ab61923a54a83e/ipld/unixfs/io/directory.go#L26

	// DefaultBatchMaxNodes controls the maximum number of nodes in a
	// write-batch. The total size of the batch is limited by
	// BatchMaxnodes and BatchMaxSize.
	DefaultBatchMaxNodes = 128
	// DefaultBatchMaxSize controls the maximum size of a single
	// write-batch. The total size of the batch is limited by
	// BatchMaxnodes and BatchMaxSize.
	DefaultBatchMaxSize = 100 << 20 // 20MiB
)

var (
	DefaultUnixFSFileMaxLinks           = int64(helpers.DefaultLinksPerBlock)
	DefaultUnixFSDirectoryMaxLinks      = int64(0)
	DefaultUnixFSHAMTDirectoryMaxFanout = int64(io.DefaultShardWidth)
)

// Import configures the default options for ingesting data. This affects commands
// that ingest data, such as 'ipfs add', 'ipfs dag put, 'ipfs block put', 'ipfs files write'.
type Import struct {
	CidVersion                       OptionalInteger
	UnixFSRawLeaves                  Flag
	UnixFSChunker                    OptionalString
	HashFunction                     OptionalString
	UnixFSFileMaxLinks               OptionalInteger
	UnixFSDirectoryMaxLinks          OptionalInteger
	UnixFSHAMTDirectoryMaxFanout     OptionalInteger
	UnixFSHAMTDirectorySizeThreshold OptionalString
	BatchMaxNodes                    OptionalInteger
	BatchMaxSize                     OptionalInteger
}
