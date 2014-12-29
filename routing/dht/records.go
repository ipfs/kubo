package dht

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	ci "github.com/jbenet/go-ipfs/crypto"
	"github.com/jbenet/go-ipfs/p2p/peer"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	u "github.com/jbenet/go-ipfs/util"
	ctxutil "github.com/jbenet/go-ipfs/util/ctx"
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

func (dht *IpfsDHT) getPublicKeyOnline(ctx context.Context, p peer.ID) (ci.PubKey, error) {
	log.Debugf("getPublicKey for: %s", p)

	// check locally.
	pk := dht.peerstore.PubKey(p)
	if pk != nil {
		return pk, nil
	}

	// ok, try the node itself. if they're overwhelmed or slow we can move on.
	ctxT, _ := ctxutil.WithDeadlineFraction(ctx, 0.3)
	if pk, err := dht.getPublicKeyFromNode(ctx, p); err == nil {
		return pk, nil
	}

	// last ditch effort: let's try the dht.
	log.Debugf("pk for %s not in peerstore, and peer failed. trying dht.", p)
	pkkey := KeyForPublicKey(p)

	// ok, try the node itself. if they're overwhelmed or slow we can move on.
	val, err := dht.GetValue(ctxT, pkkey)
	if err != nil {
		log.Warning("Failed to find requested public key.")
		return nil, err
	}

	pk, err = ci.UnmarshalPublicKey(val)
	if err != nil {
		log.Errorf("Failed to unmarshal public key: %s", err)
		return nil, err
	}
	return pk, nil
}

func (dht *IpfsDHT) getPublicKeyFromNode(ctx context.Context, p peer.ID) (ci.PubKey, error) {

	// check locally, just in case...
	pk := dht.peerstore.PubKey(p)
	if pk != nil {
		return pk, nil
	}

	pkkey := KeyForPublicKey(p)
	pmes, err := dht.getValueSingle(ctx, p, pkkey)
	if err != nil {
		return nil, err
	}

	// node doesn't have key :(
	record := pmes.GetRecord()
	if record == nil {
		return nil, fmt.Errorf("node not responding with its public key: %s", p)
	}

	// Success! We were given the value. we don't need to check
	// validity because a) we can't. b) we know the hash of the
	// key we're looking for.
	val := record.GetValue()
	log.Debug("dht got a value from other peer.")

	pk, err = ci.UnmarshalPublicKey(val)
	if err != nil {
		return nil, err
	}

	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return nil, err
	}
	if id != p {
		return nil, fmt.Errorf("public key does not match id: %s", p)
	}

	// ok! it's valid. we got it!
	log.Debugf("dht got public key from node itself.")
	return pk, nil
}

// verifyRecordLocally attempts to verify a record. if we do not have the public
// key, we fail. we do not search the dht.
func (dht *IpfsDHT) verifyRecordLocally(r *pb.Record) error {

	// First, validate the signature
	p := peer.ID(r.GetAuthor())
	pk := dht.peerstore.PubKey(p)
	if pk == nil {
		return fmt.Errorf("do not have public key for %s", p)
	}

	return dht.verifyRecord(r, pk)
}

// verifyRecordOnline verifies a record, searching the DHT for the public key
// if necessary. The reason there is a distinction in the functions is that
// retrieving arbitrary public keys from the DHT as a result of passively
// receiving records (e.g. through a PUT_VALUE or ADD_PROVIDER) can cause a
// massive amplification attack on the dht. Use with care.
func (dht *IpfsDHT) verifyRecordOnline(ctx context.Context, r *pb.Record) error {

	// get the public key, search for it if necessary.
	p := peer.ID(r.GetAuthor())
	pk, err := dht.getPublicKeyOnline(ctx, p)
	if err != nil {
		return err
	}

	return dht.verifyRecord(r, pk)
}

func (dht *IpfsDHT) verifyRecord(r *pb.Record, pk ci.PubKey) error {
	// First, validate the signature
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
