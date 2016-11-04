package blockstore

import (
	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"

	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
)

func CidToDsKey(k *cid.Cid) ds.Key {
	return dshelp.NewKeyFromBinary(k.KeyString())
}

func DsKeyToCid(dsKey ds.Key) (*cid.Cid, error) {
	kb, err := dshelp.BinaryFromDsKey(dsKey)
	if err != nil {
		return nil, err
	}
	return cid.Cast(kb)
}
