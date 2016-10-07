package dshelp

import (
	base32 "gx/ipfs/Qmb1DA2A9LS2wR4FFweB4uEDomFsdmnw1VLawLE1yQzudj/base32"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
)

// TODO: put this code into the go-datastore itself
func NewKeyFromBinary(s string) ds.Key {
	return ds.NewKey(base32.RawStdEncoding.EncodeToString([]byte(s)))
}

func BinaryFromDsKey(k ds.Key) ([]byte, error) {
	return base32.RawStdEncoding.DecodeString(k.String()[1:])
}
