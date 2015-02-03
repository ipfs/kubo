package namesys

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	pb "github.com/jbenet/go-ipfs/namesys/internal/pb"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

// ErrExpiredRecord should be returned when an ipns record is
// invalid due to being too old
var ErrExpiredRecord = errors.New("expired record")

// ErrUnrecognizedValidity is returned when an IpnsRecord has an
// unknown validity type.
var ErrUnrecognizedValidity = errors.New("unrecognized validity type")

// ipnsPublisher is capable of publishing and resolving names to the IPFS
// routing system.
type ipnsPublisher struct {
	routing routing.IpfsRouting
}

// NewRoutingPublisher constructs a publisher for the IPFS Routing name system.
func NewRoutingPublisher(route routing.IpfsRouting) Publisher {
	return &ipnsPublisher{routing: route}
}

// Publish implements Publisher. Accepts a keypair and a value,
// and publishes it out to the routing system
func (p *ipnsPublisher) Publish(ctx context.Context, k ci.PrivKey, value u.Key) error {
	log.Debugf("namesys: Publish %s", value)

	// validate `value` is a ref (multihash)
	_, err := mh.FromB58String(value.Pretty())
	if err != nil {
		return fmt.Errorf("publish value must be str multihash. %v", err)
	}

	data, err := createRoutingEntryData(k, value)
	if err != nil {
		return err
	}
	pubkey := k.GetPublic()
	pkbytes, err := pubkey.Bytes()
	if err != nil {
		return err
	}

	nameb := u.Hash(pkbytes)
	namekey := u.Key("/pk/" + string(nameb))

	log.Debugf("Storing pubkey at: %s", namekey)
	// Store associated public key
	timectx, _ := context.WithDeadline(ctx, time.Now().Add(time.Second*10))
	err = p.routing.PutValue(timectx, namekey, pkbytes)
	if err != nil {
		return err
	}

	ipnskey := u.Key("/ipns/" + string(nameb))

	log.Debugf("Storing ipns entry at: %s", ipnskey)
	// Store ipns entry at "/ipns/"+b58(h(pubkey))
	timectx, _ = context.WithDeadline(ctx, time.Now().Add(time.Second*10))
	err = p.routing.PutValue(timectx, ipnskey, data)
	if err != nil {
		return err
	}

	return nil
}

func createRoutingEntryData(pk ci.PrivKey, val u.Key) ([]byte, error) {
	entry := new(pb.IpnsEntry)

	entry.Value = []byte(val)
	typ := pb.IpnsEntry_EOL
	entry.ValidityType = &typ
	entry.Validity = []byte(u.FormatRFC3339(time.Now().Add(time.Hour * 24)))

	sig, err := pk.Sign(ipnsEntryDataForSig(entry))
	if err != nil {
		return nil, err
	}
	entry.Signature = sig
	return proto.Marshal(entry)
}

func ipnsEntryDataForSig(e *pb.IpnsEntry) []byte {
	return bytes.Join([][]byte{
		e.Value,
		e.Validity,
		[]byte(fmt.Sprint(e.GetValidityType())),
	},
		[]byte{})
}

// ValidateIpnsRecord implements ValidatorFunc and verifies that the
// given 'val' is an IpnsEntry and that that entry is valid.
func ValidateIpnsRecord(k u.Key, val []byte) error {
	entry := new(pb.IpnsEntry)
	err := proto.Unmarshal(val, entry)
	if err != nil {
		return err
	}
	switch entry.GetValidityType() {
	case pb.IpnsEntry_EOL:
		t, err := u.ParseRFC3339(string(entry.GetValidity()))
		if err != nil {
			log.Debug("Failed parsing time for ipns record EOL")
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
