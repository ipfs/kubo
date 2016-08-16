package set

import (
	"testing"

	bu "github.com/ipfs/go-ipfs/blocks/blocksutil"
	k "github.com/ipfs/go-ipfs/blocks/key"
)

const (
	tAdd int = 1 << iota
	tRemove
	tReAdd
)

func exampleKeys() []k.Key {
	res := make([]k.Key, 1<<8)
	gen := bu.NewBlockGenerator()
	for i := uint64(0); i < 1<<8; i++ {
		res[i] = gen.Next().Key()
	}
	return res
}
func checkSet(set BlockSet, keySlice []k.Key, t *testing.T) {
	for i, key := range keySlice {
		if i&tReAdd == 0 {
			if set.HasKey(key) == false {
				t.Error("key should be in the set")
			}
		} else if i&tRemove == 0 {
			if set.HasKey(key) == true {
				t.Error("key shouldn't be in the set")
			}
		} else if i&tAdd == 0 {
			if set.HasKey(key) == false {
				t.Error("key should be in the set")
			}
		}
	}
}

func TestSetWorks(t *testing.T) {
	set := NewSimpleBlockSet()
	keys := exampleKeys()

	for i, key := range keys {
		if i&tAdd == 0 {
			set.AddBlock(key)
		}
	}
	for i, key := range keys {
		if i&tRemove == 0 {
			set.RemoveBlock(key)
		}
	}
	for i, key := range keys {
		if i&tReAdd == 0 {
			set.AddBlock(key)
		}
	}

	checkSet(set, keys, t)
	addedKeys := set.GetKeys()

	newSet := SimpleSetFromKeys(addedKeys)
	// same check works on a new set
	checkSet(newSet, keys, t)

	bloom := set.GetBloomFilter()

	for _, key := range addedKeys {
		if bloom.Find([]byte(key)) == false {
			t.Error("bloom doesn't contain expected key")
		}
	}

}
