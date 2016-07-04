package blockstore

// Next to each option is it aproximate memory usage per unit
type CacheOpts struct {
	HasBloomFilterSize   int // 1 bit
	HasBloomFilterHashes int // No size, 7 is usually best, consult bloom papers
	HasARCCacheSize      int // 32 bytes
}

func DefaultCacheOpts() CacheOpts {
	return CacheOpts{
		HasBloomFilterSize:   512 * 8 * 1024,
		HasBloomFilterHashes: 7,
		HasARCCacheSize:      64 * 1024,
	}
}
