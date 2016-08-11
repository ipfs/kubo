package blockstore

import "testing"

func TestCachingOptsLessThanZero(t *testing.T) {
	opts := DefaultCacheOpts()
	opts.HasARCCacheSize = -1

	if _, err := CachedBlockstore(nil, nil, opts); err == nil {
		t.Fatal()
	}

	opts = DefaultCacheOpts()
	opts.HasBloomFilterSize = -1

	if _, err := CachedBlockstore(nil, nil, opts); err == nil {
		t.Fatal()
	}

	opts = DefaultCacheOpts()
	opts.HasBloomFilterHashes = -1

	if _, err := CachedBlockstore(nil, nil, opts); err == nil {
		t.Fatal()
	}
}

func TestBloomHashesAtZero(t *testing.T) {
	opts := DefaultCacheOpts()
	opts.HasBloomFilterHashes = 0

	if _, err := CachedBlockstore(nil, nil, opts); err == nil {
		t.Fatal()
	}
}
