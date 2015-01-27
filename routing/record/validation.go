package record

import (
	"bytes"
	"errors"
	"strings"

	ci "github.com/jbenet/go-ipfs/p2p/crypto"
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

// Validator is an object that helps ensure routing records are valid.
// It is a collection of validator functions, each of which implements
// its own notion of validity.
type Validator map[string]ValidatorFunc

// VerifyRecord checks a record and ensures it is still valid.
// It runs needed validators
func (v Validator) VerifyRecord(r *pb.Record, pk ci.PubKey) error {
	// First, validate the signature
	blob := RecordBlobForSig(r)
	ok, err := pk.Verify(blob, r.GetSignature())
	if err != nil {
		log.Info("Signature verify failed. (ignored)")
		return err
	}
	if !ok {
		log.Info("dht found a forged record! (ignored)")
		return ErrBadRecord
	}

	// Now, check validity func
	parts := strings.Split(r.GetKey(), "/")
	if len(parts) < 3 {
		log.Infof("Record key does not have validator: %s", u.Key(r.GetKey()))
		return nil
	}

	fnc, ok := v[parts[1]]
	if !ok {
		log.Infof("Unrecognized key prefix: %s", parts[1])
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
