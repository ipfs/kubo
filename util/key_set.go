package util

import (
	"sync"
)

type KeySet interface {
	Add(Key)
	Remove(Key)
	Keys() []Key
}

type ks struct {
	lock sync.RWMutex
	data map[Key]struct{}
}

func NewKeySet() KeySet {
	return &ks{
		data: make(map[Key]struct{}),
	}
}

func (wl *ks) Add(k Key) {
	wl.lock.Lock()
	defer wl.lock.Unlock()

	wl.data[k] = struct{}{}
}

func (wl *ks) Remove(k Key) {
	wl.lock.Lock()
	defer wl.lock.Unlock()

	delete(wl.data, k)
}

func (wl *ks) Keys() []Key {
	wl.lock.RLock()
	defer wl.lock.RUnlock()
	keys := make([]Key, 0)
	for k, _ := range wl.data {
		keys = append(keys, k)
	}
	return keys
}
