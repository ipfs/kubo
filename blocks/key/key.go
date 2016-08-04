package key

import (
	"encoding/json"
	"fmt"

	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	base32 "gx/ipfs/Qmb1DA2A9LS2wR4FFweB4uEDomFsdmnw1VLawLE1yQzudj/base32"
)

// Key is a string representation of multihash for use with maps.
type Key string

// String is utililty function for printing out keys as strings (Pretty).
func (k Key) String() string {
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
	return ds.NewKey(base32.RawStdEncoding.EncodeToString([]byte(k)))
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
func KeyFromDsKey(dsk ds.Key) (Key, error) {
	dec, err := base32.RawStdEncoding.DecodeString(dsk.String()[1:])
	if err != nil {
		return "", err
	}

	return Key(dec), nil
}

// KeySlice is used for sorting Keys
type KeySlice []Key

func (es KeySlice) Len() int           { return len(es) }
func (es KeySlice) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es KeySlice) Less(i, j int) bool { return es[i] < es[j] }
