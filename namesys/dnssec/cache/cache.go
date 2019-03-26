// Package cache implements a capped-size in-memory cache that randomly evicts
// elements when it reaches max size.
package cache

import (
	"crypto/rand"
	"runtime"
	"sync"
	"time"
)

type Item struct {
	Object     interface{}
	Expiration int64
}

// Returns true if the item has expired.
func (item Item) Expired() bool {
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration
}

const (
	// For use with functions that take an expiration time.
	NoExpiration time.Duration = -1
	// For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// NewFrom() when the cache was created (e.g. 5 minutes.)
	DefaultExpiration time.Duration = 0
)

type Cache struct {
	*cache
	// If this is confusing, see the comment at the bottom of New()
}

type cache struct {
	defaultExpiration time.Duration
	items             map[string]Item
	keys              *keyList
	mu                sync.RWMutex
	janitor           *janitor
}

// Add an item to the cache, replacing any existing item. If the duration is 0
// (DefaultExpiration), the cache's default expiration time is used. If it is -1
// (NoExpiration), the item never expires.
func (c *cache) Set(k string, x interface{}, d time.Duration) {
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.mu.Lock()
	if _, ok := c.items[k]; !ok {
		evicted, ok := c.keys.insert(k)
		if ok {
			delete(c.items, evicted)
		}
	}
	c.items[k] = Item{
		Object:     x,
		Expiration: e,
	}
	c.mu.Unlock()
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *cache) Get(k string) (interface{}, bool) {
	c.mu.RLock()
	item, found := c.items[k]
	c.mu.RUnlock()
	if !found {
		return nil, false
	} else if item.Expired() {
		return nil, false
	}
	return item.Object, true
}

// Delete all expired items from the cache.
func (c *cache) DeleteExpired() {
	c.mu.Lock()
	for i := 0; i < len(c.keys.keys); i++ {
		k := c.keys.keys[i]
		v, ok := c.items[k]
		if !ok {
			panic("cache inconsistent")
		}
		if v.Expired() {
			c.keys.evictAt(i)
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}

type janitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *janitor) Run(c *cache) {
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopJanitor(c *Cache) {
	c.janitor.stop <- true
}

func runJanitor(c *cache, ci time.Duration) {
	j := &janitor{
		Interval: ci,
		stop:     make(chan bool),
	}
	c.janitor = j
	go j.Run(c)
}

func newCache(de time.Duration, maxSize int, m map[string]Item) *cache {
	if de == 0 {
		de = -1
	}
	c := &cache{
		defaultExpiration: de,
		items:             m,
		keys:              &keyList{maxSize: maxSize},
	}
	return c
}

func newCacheWithJanitor(de time.Duration, ci time.Duration, maxSize int, m map[string]Item) *Cache {
	c := newCache(de, maxSize, m)
	// This trick ensures that the janitor goroutine (which--granted it
	// was enabled--is running DeleteExpired on c forever) does not keep
	// the returned C object from being garbage collected. When it is
	// garbage collected, the finalizer stops the janitor goroutine, after
	// which c can be collected.
	C := &Cache{c}
	if ci > 0 {
		runJanitor(c, ci)
		runtime.SetFinalizer(C, stopJanitor)
	}
	return C
}

// New returns a new cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration), the
// items in the cache never expire (by default), and must be deleted manually.
// If the cleanup interval is less than one, expired items are not deleted from
// the cache before calling c.DeleteExpired().
func New(defaultExpiration, cleanupInterval time.Duration, maxSize int) *Cache {
	items := make(map[string]Item)
	return newCacheWithJanitor(defaultExpiration, cleanupInterval, maxSize, items)
}

// keyList stores the list of keys in our cache in a way that is easy to
// randomly sample.
type keyList struct {
	keys    []string
	maxSize int
}

func (kl *keyList) insert(key string) (string, bool) {
	if len(kl.keys) < kl.maxSize {
		kl.keys = append(kl.keys, key)
		return "", false
	}

	// Randomly sample an index in keys.
	buff := make([]byte, 8)
	if _, err := rand.Read(buff); err != nil {
		panic(err)
	}
	var i int
	for _, b := range buff {
		i = (i << 8) | int(b)
	}
	if i < 0 {
		i = -i
	}
	i = i % len(kl.keys)

	// Replace the key at position i with the new one, return what was there.
	old := kl.keys[i]
	kl.keys[i] = key
	return old, true
}

func (kl *keyList) evictAt(i int) {
	n := len(kl.keys)

	kl.keys[i], kl.keys[n-1] = kl.keys[n-1], kl.keys[i]
	kl.keys = kl.keys[:n-1]
}
