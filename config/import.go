package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	"github.com/ipfs/boxo/ipld/unixfs/io"
	"github.com/ipfs/boxo/verifcid"
	mh "github.com/multiformats/go-multihash"
)

const (
	DefaultCidVersion      = 0
	DefaultUnixFSRawLeaves = false
	DefaultUnixFSChunker   = "size-262144"
	DefaultHashFunction    = "sha2-256"
	DefaultFastProvideRoot = true
	DefaultFastProvideWait = false

	DefaultUnixFSHAMTDirectorySizeThreshold = 262144 // 256KiB - https://github.com/ipfs/boxo/blob/6c5a07602aed248acc86598f30ab61923a54a83e/ipld/unixfs/io/directory.go#L26

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
	UnixFSHAMTDirectorySizeThreshold OptionalBytes
	BatchMaxNodes                    OptionalInteger
	BatchMaxSize                     OptionalInteger
	FastProvideRoot                  Flag
	FastProvideWait                  Flag
}

// ValidateImportConfig validates the Import configuration according to UnixFS spec requirements.
// See: https://specs.ipfs.tech/unixfs/#hamt-structure-and-parameters
func ValidateImportConfig(cfg *Import) error {
	// Validate CidVersion
	if !cfg.CidVersion.IsDefault() {
		cidVer := cfg.CidVersion.WithDefault(DefaultCidVersion)
		if cidVer != 0 && cidVer != 1 {
			return fmt.Errorf("Import.CidVersion must be 0 or 1, got %d", cidVer)
		}
	}

	// Validate UnixFSFileMaxLinks
	if !cfg.UnixFSFileMaxLinks.IsDefault() {
		maxLinks := cfg.UnixFSFileMaxLinks.WithDefault(DefaultUnixFSFileMaxLinks)
		if maxLinks <= 0 {
			return fmt.Errorf("Import.UnixFSFileMaxLinks must be positive, got %d", maxLinks)
		}
	}

	// Validate UnixFSDirectoryMaxLinks
	if !cfg.UnixFSDirectoryMaxLinks.IsDefault() {
		maxLinks := cfg.UnixFSDirectoryMaxLinks.WithDefault(DefaultUnixFSDirectoryMaxLinks)
		if maxLinks < 0 {
			return fmt.Errorf("Import.UnixFSDirectoryMaxLinks must be non-negative, got %d", maxLinks)
		}
	}

	// Validate UnixFSHAMTDirectoryMaxFanout if set
	if !cfg.UnixFSHAMTDirectoryMaxFanout.IsDefault() {
		fanout := cfg.UnixFSHAMTDirectoryMaxFanout.WithDefault(DefaultUnixFSHAMTDirectoryMaxFanout)

		// Check all requirements: fanout < 8 covers both non-positive and non-multiple of 8
		// Combined with power of 2 check and max limit, this ensures valid values: 8, 16, 32, 64, 128, 256, 512, 1024
		if fanout < 8 || !isPowerOfTwo(fanout) || fanout > 1024 {
			return fmt.Errorf("Import.UnixFSHAMTDirectoryMaxFanout must be a positive power of 2, multiple of 8, and not exceed 1024 (got %d)", fanout)
		}
	}

	// Validate BatchMaxNodes
	if !cfg.BatchMaxNodes.IsDefault() {
		maxNodes := cfg.BatchMaxNodes.WithDefault(DefaultBatchMaxNodes)
		if maxNodes <= 0 {
			return fmt.Errorf("Import.BatchMaxNodes must be positive, got %d", maxNodes)
		}
	}

	// Validate BatchMaxSize
	if !cfg.BatchMaxSize.IsDefault() {
		maxSize := cfg.BatchMaxSize.WithDefault(DefaultBatchMaxSize)
		if maxSize <= 0 {
			return fmt.Errorf("Import.BatchMaxSize must be positive, got %d", maxSize)
		}
	}

	// Validate UnixFSChunker format
	if !cfg.UnixFSChunker.IsDefault() {
		chunker := cfg.UnixFSChunker.WithDefault(DefaultUnixFSChunker)
		if !isValidChunker(chunker) {
			return fmt.Errorf("Import.UnixFSChunker invalid format: %q (expected \"size-<bytes>\", \"rabin-<min>-<avg>-<max>\", or \"buzhash\")", chunker)
		}
	}

	// Validate HashFunction
	if !cfg.HashFunction.IsDefault() {
		hashFunc := cfg.HashFunction.WithDefault(DefaultHashFunction)
		hashCode, ok := mh.Names[strings.ToLower(hashFunc)]
		if !ok {
			return fmt.Errorf("Import.HashFunction unrecognized: %q", hashFunc)
		}
		// Check if the hash is allowed by verifcid
		if !verifcid.DefaultAllowlist.IsAllowed(hashCode) {
			return fmt.Errorf("Import.HashFunction %q is not allowed for use in IPFS", hashFunc)
		}
	}

	return nil
}

// isPowerOfTwo checks if a number is a power of 2
func isPowerOfTwo(n int64) bool {
	return n > 0 && (n&(n-1)) == 0
}

// isValidChunker validates chunker format
func isValidChunker(chunker string) bool {
	if chunker == "buzhash" {
		return true
	}

	// Check for size-<bytes> format
	if strings.HasPrefix(chunker, "size-") {
		sizeStr := strings.TrimPrefix(chunker, "size-")
		if sizeStr == "" {
			return false
		}
		// Check if it's a valid positive integer (no negative sign allowed)
		if sizeStr[0] == '-' {
			return false
		}
		size, err := strconv.Atoi(sizeStr)
		// Size must be positive (not zero)
		return err == nil && size > 0
	}

	// Check for rabin-<min>-<avg>-<max> format
	if strings.HasPrefix(chunker, "rabin-") {
		parts := strings.Split(chunker, "-")
		if len(parts) != 4 {
			return false
		}

		// Parse and validate min, avg, max values
		values := make([]int, 3)
		for i := 0; i < 3; i++ {
			val, err := strconv.Atoi(parts[i+1])
			if err != nil {
				return false
			}
			values[i] = val
		}

		// Validate ordering: min <= avg <= max
		min, avg, max := values[0], values[1], values[2]
		return min <= avg && avg <= max
	}

	return false
}
