package namesys

import (
	"time"

	path "github.com/ipfs/go-path"
)

func (ns *mpns) cacheGet(name string) (path.Path, [][]byte, bool) {
	if ns.cache == nil {
		return "", nil, false
	}

	ientry, ok := ns.cache.Get(name)
	if !ok {
		return "", nil, false
	}

	entry, ok := ientry.(cacheEntry)
	if !ok {
		// should never happen, purely for sanity
		log.Panicf("unexpected type %T in cache for %q.", ientry, name)
	}

	if time.Now().Before(entry.eol) {
		return entry.val, entry.proof, true
	}

	ns.cache.Remove(name)

	return "", nil, false
}

func (ns *mpns) cacheSet(name string, val path.Path, proof [][]byte, ttl time.Duration) {
	if ns.cache == nil || ttl <= 0 {
		return
	}
	ns.cache.Add(name, cacheEntry{
		val:   val,
		proof: proof,
		eol:   time.Now().Add(ttl),
	})
}

type cacheEntry struct {
	val   path.Path
	proof [][]byte
	eol   time.Time
}
