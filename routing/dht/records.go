package dht

import (
	"bytes"
	"errors"
	"strings"

	"code.google.com/p/goprotobuf/proto"
	"github.com/jbenet/go-ipfs/peer"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	u "github.com/jbenet/go-ipfs/util"
)

type ValidatorFunc func(u.Key, []byte) error

var ErrBadRecord = errors.New("bad dht record")
var ErrInvalidRecordType = errors.New("invalid record keytype")

// creates and signs a dht record for the given key/value pair
func (dht *IpfsDHT) makePutRecord(key u.Key, value []byte) (*pb.Record, error) {
	record := new(pb.Record)

	record.Key = proto.String(string(key))
	record.Value = value
	record.Author = proto.String(string(dht.self.ID()))
	blob := bytes.Join([][]byte{[]byte(key), value, []byte(dht.self.ID())}, []byte{})
	sig, err := dht.self.PrivKey().Sign(blob)
	if err != nil {
		return nil, err
	}
	record.Signature = sig
	return record, nil
}

func (dht *IpfsDHT) verifyRecord(r *pb.Record) error {
	// First, validate the signature
	p, err := dht.peerstore.Get(peer.ID(r.GetAuthor()))
	if err != nil {
		return err
	}
	k := u.Key(r.GetKey())

	blob := bytes.Join([][]byte{[]byte(k),
		r.GetValue(),
		[]byte(r.GetAuthor())}, []byte{})

	ok, err := p.PubKey().Verify(blob, r.GetSignature())
	if err != nil {
		log.Error("Signature verify failed.")
		return err
	}

	if !ok {
		return ErrBadRecord
	}

	// Now, check validity func
	parts := strings.Split(r.GetKey(), "/")
	if len(parts) < 3 {
		log.Errorf("Record had bad key: %s", u.Key(r.GetKey()))
		return ErrBadRecord
	}

	fnc, ok := dht.Validators[parts[1]]
	if !ok {
		log.Errorf("Unrecognized key prefix: %s", parts[1])
		return ErrInvalidRecordType
	}

	return fnc(u.Key(r.GetKey()), r.GetValue())
}

func ValidateIpnsRecord(k u.Key, val []byte) error {
	// TODO:
	return nil
}

func ValidatePublicKeyRecord(k u.Key, val []byte) error {
	// TODO:
	return nil
}
