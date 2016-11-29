package dshelp

import (
	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	base32 "gx/ipfs/QmZvZSVtvxak4dcTkhsQhqd1SQ6rg5UzaSTu62WfWKjj93/base32"
	cid "gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
)

// TODO: put this code into the go-datastore itself
func NewKeyFromBinary(s string) ds.Key {
	return ds.NewKey(base32.RawStdEncoding.EncodeToString([]byte(s)))
}

func BinaryFromDsKey(k ds.Key) ([]byte, error) {
	return base32.RawStdEncoding.DecodeString(k.String()[1:])
}

func CidToDsKey(k *cid.Cid) ds.Key {
	return NewKeyFromBinary(k.KeyString())
}

func DsKeyToCid(dsKey ds.Key) (*cid.Cid, error) {
	kb, err := BinaryFromDsKey(dsKey)
	if err != nil {
		return nil, err
	}
	return cid.Cast(kb)
}

func DsKeyStringToCid(dsKey string) (*cid.Cid, error) {
	kb, err := base32.RawStdEncoding.DecodeString(dsKey[1:])
	if err != nil {
		return nil, err
	}
	return cid.Cast(kb)
}
