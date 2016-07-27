package record

import (
	"bytes"

	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"

	key "github.com/ipfs/go-ipfs/blocks/key"
	pb "github.com/ipfs/go-ipfs/routing/dht/pb"
	logging "gx/ipfs/QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR/go-log"
	ci "gx/ipfs/QmUWER4r4qMvaCnX5zREcfyiWN7cXN9g3a7fkRqNz8qWPP/go-libp2p-crypto"
)

var log = logging.Logger("routing/record")

// MakePutRecord creates and signs a dht record for the given key/value pair
func MakePutRecord(sk ci.PrivKey, key key.Key, value []byte, sign bool) (*pb.Record, error) {
	record := new(pb.Record)

	record.Key = proto.String(string(key))
	record.Value = value

	pkh, err := sk.GetPublic().Hash()
	if err != nil {
		return nil, err
	}

	record.Author = proto.String(string(pkh))
	if sign {
		blob := RecordBlobForSig(record)

		sig, err := sk.Sign(blob)
		if err != nil {
			return nil, err
		}

		record.Signature = sig
	}
	return record, nil
}

// RecordBlobForSig returns the blob protected by the record signature
func RecordBlobForSig(r *pb.Record) []byte {
	k := []byte(r.GetKey())
	v := []byte(r.GetValue())
	a := []byte(r.GetAuthor())
	return bytes.Join([][]byte{k, v, a}, []byte{})
}
