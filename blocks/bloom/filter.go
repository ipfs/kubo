// package bloom implements a simple bloom filter.
package bloom

import (
	"errors"
	"fmt"
	"hash"
	"hash/adler32"
	"hash/crc32"
	"hash/fnv"
	"math/big"
)

type Filter interface {
	Add([]byte)
	Find([]byte) bool
	Merge(Filter) (Filter, error)
}

func BasicFilter() Filter {
	// Non crypto hashes, because speed
	return NewFilter(2048, adler32.New(), fnv.New32(), crc32.NewIEEE())
}

func NewFilter(size int, hashes ...hash.Hash) Filter {
	return &filter{
		filter: make([]byte, size),
		hashes: hashes,
	}
}

type filter struct {
	filter []byte
	hashes []hash.Hash
}

func (f *filter) Add(k []byte) {
	for _, h := range f.hashes {
		i := bytesMod(h.Sum(k), int64(len(f.filter)*8))
		f.setBit(i)
	}
}

func (f *filter) Find(k []byte) bool {
	for _, h := range f.hashes {
		i := bytesMod(h.Sum(k), int64(len(f.filter)*8))
		if !f.getBit(i) {
			return false
		}
	}
	return true
}

func (f *filter) setBit(i int64) {
	fmt.Printf("setting bit %d\n", i)
	f.filter[i/8] |= (1 << byte(i%8))
}

func (f *filter) getBit(i int64) bool {
	fmt.Printf("getting bit %d\n", i)
	return f.filter[i/8]&(1<<byte(i%8)) != 0
}

func bytesMod(b []byte, modulo int64) int64 {
	i := big.NewInt(0)
	i = i.SetBytes(b)

	bigmod := big.NewInt(int64(modulo))
	result := big.NewInt(0)
	result.Mod(i, bigmod)

	return result.Int64()
}

func (f *filter) Merge(o Filter) (Filter, error) {
	casfil, ok := o.(*filter)
	if !ok {
		return nil, errors.New("Unsupported filter type")
	}

	if len(casfil.filter) != len(f.filter) {
		return nil, errors.New("filter lengths must match!")
	}

	nfilt := new(filter)

	// this bit is sketchy, need a way of comparing hash functions
	nfilt.hashes = f.hashes

	nfilt.filter = make([]byte, len(f.filter))
	for i, v := range f.filter {
		nfilt.filter[i] = v | casfil.filter[i]
	}

	return nfilt, nil
}
