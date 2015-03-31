package util

import (
	"encoding/json"
	"fmt"

	b58 "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	mh "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
)

// Key is a string representation of multihash for use with maps.
type Key string

// String is utililty function for printing out keys as strings (Pretty).
func (k Key) String() string {
	return k.Pretty()
}

// Pretty returns Key in a b58 encoded string
// TODO: deprecate Pretty. bad name.
func (k Key) Pretty() string {
	return k.B58String()
}

func (k Key) ToMultihash() mh.Multihash {
	return mh.Multihash(k)
}

// B58String returns Key in a b58 encoded string
func (k Key) B58String() string {
	return B58KeyEncode(k)
}

// B58KeyDecode returns Key from a b58 encoded string
func B58KeyDecode(s string) Key {
	return Key(string(b58.Decode(s)))
}

// B58KeyEncode returns Key in a b58 encoded string
func B58KeyEncode(k Key) string {
	return b58.Encode([]byte(k))
}

// DsKey returns a Datastore key
func (k Key) DsKey() ds.Key {
	return ds.NewKey(string(k))
}

// UnmarshalJSON returns a JSON-encoded Key (string)
func (k *Key) UnmarshalJSON(mk []byte) error {
	var s string
	err := json.Unmarshal(mk, &s)
	if err != nil {
		return err
	}

	*k = Key(string(b58.Decode(s)))
	if len(*k) == 0 && len(s) > 2 { // if b58.Decode fails, k == ""
		return fmt.Errorf("Key.UnmarshalJSON: invalid b58 string: %v", mk)
	}
	return nil
}

// MarshalJSON returns a JSON-encoded Key (string)
func (k *Key) MarshalJSON() ([]byte, error) {
	return json.Marshal(b58.Encode([]byte(*k)))
}

func (k *Key) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"key": k.String(),
	}
}

// KeyFromDsKey returns a Datastore key
func KeyFromDsKey(dsk ds.Key) Key {
	return Key(dsk.String()[1:])
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
	_, err := mh.Cast(out)
	if err != nil {
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

// KeySlice is used for sorting Keys
type KeySlice []Key

func (es KeySlice) Len() int           { return len(es) }
func (es KeySlice) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es KeySlice) Less(i, j int) bool { return es[i] < es[j] }
