package namesys

import (
	"errors"
	"time"

	pb "github.com/ipfs/go-ipfs/namesys/pb"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	pstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	record "gx/ipfs/QmUpttFinNDmNPgFwKN8sZK6BUtBmA68Y4KdSBDXa8t9sJ/go-libp2p-record"
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

func NewIpnsRecordValidator(kbook pstore.KeyBook) *record.ValidChecker {
	// ValidateIpnsRecord implements ValidatorFunc and verifies that the
	// given 'val' is an IpnsEntry and that that entry is valid.
	ValidateIpnsRecord := func(r *record.ValidationRecord) error {
		if r.Namespace != "ipns" {
			return ErrInvalidPath
		}

		// Parse the value into an IpnsEntry
		entry := new(pb.IpnsEntry)
		err := proto.Unmarshal(r.Value, entry)
		if err != nil {
			return err
		}

		// Get the public key defined by the ipns path
		pid, err := peer.IDFromString(r.Key)
		if err != nil {
			log.Debugf("failed to parse ipns record key %s into public key hash", r.Key)
			return ErrSignature
		}
		pubk := kbook.PubKey(pid)
		if pubk == nil {
			log.Debugf("public key with hash %s not found in peer store", pid)
			return ErrSignature
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
