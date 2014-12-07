// Package mpool provides a sync.Pool equivalent that buckets incoming
// requests to one of 32 sub-pools, one for each power of 2, 0-32.
//
//	import "github.com/jbenet/go-msgio/mpool"
//	var p mpool.Pool
//
//	small := make([]byte, 1024)
//	large := make([]byte, 4194304)
//	p.Put(1024, small)
//	p.Put(4194304, large)
//
//	small2 := p.Get(1024).([]byte)
//	large2 := p.Get(4194304).([]byte)
//	fmt.Println("small2 len:", len(small2))
//	fmt.Println("large2 len:", len(large2))
//
//	// Output:
//	// small2 len: 1024
//	// large2 len: 4194304
//
package mpool

import (
	"fmt"
	"sync"
)

// ByteSlicePool is a static Pool for reusing byteslices of various sizes.
var ByteSlicePool Pool

func init() {
	ByteSlicePool.New = func(length int) interface{} {
		return make([]byte, length)
	}
}

// MaxLength is the maximum length of an element that can be added to the Pool.
const MaxLength = (1 << 32) - 1

// Pool is a pool to handle cases of reusing elements of varying sizes.
// It maintains up to  32 internal pools, for each power of 2 in 0-32.
type Pool struct {
	small      int            // the size of the first pool
	pools      [32]*sync.Pool // a list of singlePools
	sync.Mutex                // protecting list

	// New is a function that constructs a new element in the pool, with given len
	New func(len int) interface{}
}

func (p *Pool) getPool(idx uint32) *sync.Pool {
	if idx > uint32(len(p.pools)) {
		panic(fmt.Errorf("index too large: %d", idx))
	}

	p.Lock()
	defer p.Unlock()

	sp := p.pools[idx]
	if sp == nil {
		sp = new(sync.Pool)
		p.pools[idx] = sp
	}
	return sp
}

// Get selects an arbitrary item from the Pool, removes it from the Pool,
// and returns it to the caller. Get may choose to ignore the pool and
// treat it as empty. Callers should not assume any relation between values
// passed to Put and the values returned by Get.
//
// If Get would otherwise return nil and p.New is non-nil, Get returns the
// result of calling p.New.
func (p *Pool) Get(length uint32) interface{} {
	idx := largerPowerOfTwo(length)
	sp := p.getPool(idx)
	val := sp.Get()
	if val == nil && p.New != nil {
		val = p.New(0x1 << idx)
	}
	return val
}

// Put adds x to the pool.
func (p *Pool) Put(length uint32, val interface{}) {
	if length > MaxLength {
		length = MaxLength
	}

	idx := smallerPowerOfTwo(length)
	sp := p.getPool(idx)
	sp.Put(val)
}

func largerPowerOfTwo(num uint32) uint32 {
	for p := uint32(0); p < 32; p++ {
		if (0x1 << p) >= num {
			return p
		}
	}

	panic("unreachable")
}

func smallerPowerOfTwo(num uint32) uint32 {
	for p := uint32(1); p < 32; p++ {
		if (0x1 << p) > num {
			return p - 1
		}
	}

	panic("unreachable")
}
