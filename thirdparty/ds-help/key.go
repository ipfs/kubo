package dshelp

import (
	ds "gx/ipfs/QmdHG8MAuARdGHxx4rPQASLcvhz24fzjSQq7AJRAQEorq5/go-datastore"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
	base32 "gx/ipfs/QmfVj3x4D6Jkq9SEoi5n2NmoUomLwoeiwnYz2KQa15wRw6/base32"
)

// TODO: put this code into the go-datastore itself

func NewKeyFromBinary(rawKey []byte) ds.Key {
	buf := make([]byte, 1+base32.RawStdEncoding.EncodedLen(len(rawKey)))
	buf[0] = '/'
	base32.RawStdEncoding.Encode(buf[1:], rawKey)
	return ds.RawKey(string(buf))
}

func BinaryFromDsKey(k ds.Key) ([]byte, error) {
	return base32.RawStdEncoding.DecodeString(k.String()[1:])
}

func CidToDsKey(k *cid.Cid) ds.Key {
	return NewKeyFromBinary(k.Bytes())
}

func DsKeyToCid(dsKey ds.Key) (*cid.Cid, error) {
	kb, err := BinaryFromDsKey(dsKey)
	if err != nil {
		return nil, err
	}
	return cid.Cast(kb)
}
