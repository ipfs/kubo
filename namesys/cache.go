package namesys

import (
	"time"

	path "github.com/ipfs/go-path"
)

func (ns *mpns) cacheGet(name string) (path.Path, bool) {
	// existence of optional mapping defined via IPFS_NS_MAP is checked first
	if ns.staticMap != nil {
		val, ok := ns.staticMap[name]
		if ok {
			return val, true
		}
	}

	if ns.cache == nil {
		return "", false
	}

	ientry, ok := ns.cache.Get(name)
	if !ok {
		return "", false
	}

	entry, ok := ientry.(cacheEntry)
	if !ok {
		// should never happen, purely for sanity
		log.Panicf("unexpected type %T in cache for %q.", ientry, name)
	}

	if time.Now().Before(entry.eol) {
		return entry.val, true
	}

	ns.cache.Remove(name)

	return "", false
}

func (ns *mpns) cacheSet(name string, val path.Path, ttl time.Duration) {
	if ns.cache == nil || ttl <= 0 {
		return
	}
	ns.cache.Add(name, cacheEntry{
		val: val,
		eol: time.Now().Add(ttl),
	})
}

func (ns *mpns) cacheInvalidate(name string) {
	if ns.cache == nil {
		return
	}
	ns.cache.Remove(name)
}

type cacheEntry struct {
	val path.Path
	eol time.Time
}
