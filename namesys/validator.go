package namesys

import (
	"errors"
	"time"

	pb "github.com/ipfs/go-ipfs/namesys/pb"
	pstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	record "gx/ipfs/QmUpttFinNDmNPgFwKN8sZK6BUtBmA68Y4KdSBDXa8t9sJ/go-libp2p-record"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
)

// ErrExpiredRecord should be returned when an ipns record is
// invalid due to being too old
var ErrExpiredRecord = errors.New("expired record")

// ErrUnrecognizedValidity is returned when an IpnsRecord has an
// unknown validity type.
var ErrUnrecognizedValidity = errors.New("unrecognized validity type")

// ErrInvalidPath should be returned when an ipns record path
// is not in a valid format
var ErrInvalidPath = errors.New("record path invalid")

// ErrSignature should be returned when an ipns record fails
// signature verification
var ErrSignature = errors.New("record signature verification failed")

// ErrBadRecord should be returned when an ipns record cannot be unmarshalled
var ErrBadRecord = errors.New("record could not be unmarshalled")

// ErrKeyFormat should be returned when an ipns record key is
// incorrectly formatted (not a peer ID)
var ErrKeyFormat = errors.New("record key could not be parsed into peer ID")

// ErrPublicKeyNotFound should be returned when the public key
// corresponding to the ipns record path cannot be retrieved
// from the peer store
var ErrPublicKeyNotFound = errors.New("public key not found in peer store")

// NewIpnsRecordValidator returns a ValidChecker for IPNS records.
// The validator function will get a public key from the KeyBook
// to verify the record's signature. Note that the public key must
// already have been fetched from the network and put into the KeyBook
// by the caller.
func NewIpnsRecordValidator(kbook pstore.KeyBook) *record.ValidChecker {
	// ValidateIpnsRecord implements ValidatorFunc and verifies that the
	// given record's value is an IpnsEntry, that the entry has been correctly
	// signed, and that the entry has not expired
	ValidateIpnsRecord := func(r *record.ValidationRecord) error {
		if r.Namespace != "ipns" {
			return ErrInvalidPath
		}

		// Parse the value into an IpnsEntry
		entry := new(pb.IpnsEntry)
		err := proto.Unmarshal(r.Value, entry)
		if err != nil {
			return ErrBadRecord
		}

		// Get the public key defined by the ipns path
		pid, err := peer.IDFromString(r.Key)
		if err != nil {
			log.Debugf("failed to parse ipns record key %s into peer ID", r.Key)
			return ErrKeyFormat
		}
		pubk := kbook.PubKey(pid)
		if pubk == nil {
			log.Debugf("public key with hash %s not found in peer store", pid)
			return ErrPublicKeyNotFound
		}

		// Check the ipns record signature with the public key
		if ok, err := pubk.Verify(ipnsEntryDataForSig(entry), entry.GetSignature()); err != nil || !ok {
			log.Debugf("failed to verify signature for ipns record %s", r.Key)
			return ErrSignature
		}

		// Check that record has not expired
		switch entry.GetValidityType() {
		case pb.IpnsEntry_EOL:
			t, err := u.ParseRFC3339(string(entry.GetValidity()))
			if err != nil {
				log.Debugf("failed parsing time for ipns record EOL in record %s", r.Key)
				return err
			}
			if time.Now().After(t) {
				return ErrExpiredRecord
			}
		default:
			return ErrUnrecognizedValidity
		}
		return nil
	}

	return &record.ValidChecker{
		Func: ValidateIpnsRecord,
		Sign: false,
	}
}
