package blockstore

import (
	"errors"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

// Next to each option is it aproximate memory usage per unit
type CacheOpts struct {
	HasBloomFilterSize   int // 1 bit
	HasBloomFilterHashes int // No size, 7 is usually best, consult bloom papers
	HasARCCacheSize      int // 32 bytes
	BlockARCCacheSize    int // 512KiB max
}

func DefaultCacheOpts() CacheOpts {
	return CacheOpts{
		256 * 1024,
		7,
		64 * 1024,
		16,
	}
}

func CachedBlockstore(bs GCBlockstore,
	ctx context.Context, opts CacheOpts) (cbs GCBlockstore, err error) {
	if ctx == nil {
		ctx = context.TODO() // For tests
	}
	cbs = bs

	if opts.HasBloomFilterSize < 0 || opts.HasBloomFilterHashes < 0 ||
		opts.HasARCCacheSize < 0 || opts.BlockARCCacheSize < 0 {
		return nil, errors.New("all options for cache need to be greater than zero")
	}

	if opts.HasBloomFilterSize != 0 && opts.HasBloomFilterHashes == 0 {
		return nil, errors.New("bloom filter hash count can't be 0 when there is size set")
	}
	if opts.HasBloomFilterSize != 0 {
		cbs, err = bloomCached(cbs, ctx, opts.HasBloomFilterSize, opts.HasBloomFilterHashes,
			opts.HasARCCacheSize)
	}

	return cbs, err
}
