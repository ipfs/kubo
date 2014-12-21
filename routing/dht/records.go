package dht

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	ci "github.com/jbenet/go-ipfs/crypto"
	"github.com/jbenet/go-ipfs/peer"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	u "github.com/jbenet/go-ipfs/util"
)

// ValidatorFunc is a function that is called to validate a given
// type of DHTRecord.
type ValidatorFunc func(u.Key, []byte) error

// ErrBadRecord is returned any time a dht record is found to be
// incorrectly formatted or signed.
var ErrBadRecord = errors.New("bad dht record")

// ErrInvalidRecordType is returned if a DHTRecord keys prefix
// is not found in the Validator map of the DHT.
var ErrInvalidRecordType = errors.New("invalid record keytype")

// KeyForPublicKey returns the key used to retrieve public keys
// from the dht.
func KeyForPublicKey(id peer.ID) u.Key {
	return u.Key("/pk/" + string(id))
}

// RecordBlobForSig returns the blob protected by the record signature
func RecordBlobForSig(r *pb.Record) []byte {
	k := []byte(r.GetKey())
	v := []byte(r.GetValue())
	a := []byte(r.GetAuthor())
	return bytes.Join([][]byte{k, v, a}, []byte{})
}

// creates and signs a dht record for the given key/value pair
func (dht *IpfsDHT) makePutRecord(key u.Key, value []byte) (*pb.Record, error) {
	record := new(pb.Record)

	record.Key = proto.String(string(key))
	record.Value = value
	record.Author = proto.String(string(dht.self))
	blob := RecordBlobForSig(record)

	sk := dht.peerstore.PrivKey(dht.self)
	if sk == nil {
		log.Errorf("%s dht cannot get own private key!", dht.self)
		return nil, fmt.Errorf("cannot get private key to sign record!")
	}

	sig, err := sk.Sign(blob)
	if err != nil {
		return nil, err
	}

	record.Signature = sig
	return record, nil
}

func (dht *IpfsDHT) getPublicKey(p peer.ID) (ci.PubKey, error) {
	log.Debug("getPublicKey for: %s", p)

	// check locally.
	pk := dht.peerstore.PubKey(p)
	if pk != nil {
		return pk, nil
	}

	log.Debug("not in peerstore, searching dht.")
	ctxT, _ := context.WithTimeout(dht.ContextGroup.Context(), time.Second*5)
	val, err := dht.GetValue(ctxT, KeyForPublicKey(p))
	if err != nil {
		log.Warning("Failed to find requested public key.")
		return nil, err
	}

	pubkey, err := ci.UnmarshalPublicKey(val)
	if err != nil {
		log.Errorf("Failed to unmarshal public key: %s", err)
		return nil, err
	}
	return pubkey, nil
}

func (dht *IpfsDHT) verifyRecord(r *pb.Record) error {
	// First, validate the signature
	p := peer.ID(r.GetAuthor())
	pk, err := dht.getPublicKey(p)
	if err != nil {
		return err
	}

	blob := RecordBlobForSig(r)
	ok, err := pk.Verify(blob, r.GetSignature())
	if err != nil {
		log.Error("Signature verify failed.")
		return err
	}
	if !ok {
		log.Error("dht found a forged record! (ignored)")
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

// ValidatePublicKeyRecord implements ValidatorFunc and
// verifies that the passed in record value is the PublicKey
// that matches the passed in key.
func ValidatePublicKeyRecord(k u.Key, val []byte) error {
	keyparts := bytes.Split([]byte(k), []byte("/"))
	if len(keyparts) < 3 {
		return errors.New("invalid key")
	}

	pkh := u.Hash(val)
	if !bytes.Equal(keyparts[2], pkh) {
		return errors.New("public key does not match storage key")
	}
	return nil
}
