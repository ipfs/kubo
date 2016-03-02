package filecache

import (
	"math/rand"
	"testing"
	"time"

	"gx/ipfs/QmYWL7Pyx6QHHryhLq96wR6CWidApH2D2nbXeTJbAmusH9/iter"
)

func BenchmarkInsert(b *testing.B) {
	for range iter.N(b.N) {
		li := newLRUItems()
		for range iter.N(10000) {
			r := rand.Int63()
			t := time.Unix(r/1e9, r%1e9)
			li.Insert(ItemInfo{
				Accessed: t,
			})
		}
	}
}
