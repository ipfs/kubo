package blockstore

// Next to each option is it aproximate memory usage per unit
type CacheOpts struct {
	HasBloomFilterSize   uint // 1 bit
	HasBloomFilterHashes uint // No size, 7 is usually best, consult bloom papers
	HasARCCacheSize      uint // 32 bytes
	BlockARCCacheSize    uint // 512KiB max
}

func DefaultCacheOpts() CacheOpts {
	return CacheOpts{
		256 * 1024,
		7,
		64 * 1024,
		16,
	}
}
