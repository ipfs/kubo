package util

import (
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
)

// Key is a string representation of multihash for use with maps.
type Key string

// String is utililty function for printing out keys as strings (Pretty).
func (k Key) String() string {
	return k.Pretty()
}

// Pretty returns Key in a b58 encoded string
func (k Key) Pretty() string {
	return b58.Encode([]byte(k))
}

// DsKey returns a Datastore key
func (k Key) DsKey() ds.Key {
	return ds.NewKey(string(k))
}

// KeyFromDsKey returns a Datastore key
func KeyFromDsKey(dsk ds.Key) Key {
	return Key(dsk.BaseNamespace())
}

// B58KeyConverter -- for KeyTransform datastores
// (static as only one obj needed)
var B58KeyConverter = b58KeyConverter{}

type b58KeyConverter struct{}

// ConvertKey returns a B58 encoded Datastore key
// TODO: this is hacky because it encodes every path component. some
// path components may be proper strings already...
func (b58KeyConverter) ConvertKey(dsk ds.Key) ds.Key {
	k := ds.NewKey("/")
	for _, n := range dsk.Namespaces() {
		k = k.ChildString(b58.Encode([]byte(n)))
	}
	return k
}

// InvertKey returns a b58 decoded Datastore key
// TODO: this is hacky because it encodes every path component. some
// path components may be proper strings already...
func (b58KeyConverter) InvertKey(dsk ds.Key) ds.Key {
	k := ds.NewKey("/")
	for _, n := range dsk.Namespaces() {
		k = k.ChildString(string(b58.Decode(n)))
	}
	return k
}

// Hash is the global IPFS hash function. uses multihash SHA2_256, 256 bits
func Hash(data []byte) mh.Multihash {
	h, err := mh.Sum(data, mh.SHA2_256, -1)
	if err != nil {
		// this error can be safely ignored (panic) because multihash only fails
		// from the selection of hash function. If the fn + length are valid, it
		// won't error.
		panic("multihash failed to hash using SHA2_256.")
	}
	return h
}

// IsValidHash checks whether a given hash is valid (b58 decodable, len > 0)
func IsValidHash(s string) bool {
	out := b58.Decode(s)
	if out == nil || len(out) == 0 {
		return false
	}
	return true
}

// XOR takes two byte slices, XORs them together, returns the resulting slice.
func XOR(a, b []byte) []byte {
	c := make([]byte, len(a))
	for i := 0; i < len(a); i++ {
		c[i] = a[i] ^ b[i]
	}
	return c
}
