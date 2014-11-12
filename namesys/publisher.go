package namesys

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	ci "github.com/jbenet/go-ipfs/crypto"
	pb "github.com/jbenet/go-ipfs/namesys/internal/pb"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

// ErrExpiredRecord should be returned when an ipns record is
// invalid due to being too old
var ErrExpiredRecord = errors.New("expired record")

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
func (p *ipnsPublisher) Publish(k ci.PrivKey, value string) error {
	log.Debugf("namesys: Publish %s", value)

	// validate `value` is a ref (multihash)
	_, err := mh.FromB58String(value)
	if err != nil {
		log.Errorf("hash cast failed: %s", value)
		return fmt.Errorf("publish value must be str multihash. %v", err)
	}

	ctx := context.TODO()
	data, err := createRoutingEntryData(k, value)
	if err != nil {
		log.Error("entry creation failed.")
		return err
	}
	pubkey := k.GetPublic()
	pkbytes, err := pubkey.Bytes()
	if err != nil {
		log.Error("pubkey getbytes failed.")
		return err
	}

	nameb := u.Hash(pkbytes)
	namekey := u.Key("/pk/" + string(nameb))

	log.Debugf("Storing pubkey at: %s", namekey)
	// Store associated public key
	timectx, _ := context.WithDeadline(ctx, time.Now().Add(time.Second*4))
	err = p.routing.PutValue(timectx, namekey, pkbytes)
	if err != nil {
		return err
	}

	ipnskey := u.Key("/ipns/" + string(nameb))

	log.Debugf("Storing ipns entry at: %s", ipnskey)
	// Store ipns entry at "/ipns/"+b58(h(pubkey))
	timectx, _ = context.WithDeadline(ctx, time.Now().Add(time.Second*4))
	err = p.routing.PutValue(timectx, ipnskey, data)
	if err != nil {
		return err
	}

	return nil
}

func createRoutingEntryData(pk ci.PrivKey, val string) ([]byte, error) {
	entry := new(pb.IpnsEntry)

	entry.Value = []byte(val)
	typ := pb.IpnsEntry_EOL
	entry.ValidityType = &typ
	entry.Validity = []byte(time.Now().Add(time.Hour * 24).String())

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

func ValidateIpnsRecord(k u.Key, val []byte) error {
	entry := new(pb.IpnsEntry)
	err := proto.Unmarshal(val, entry)
	if err != nil {
		return err
	}
	switch entry.GetValidityType() {
	case pb.IpnsEntry_EOL:
		defaultTimeFormat := "2006-01-02 15:04:05.999999999 -0700 MST"
		t, err := time.Parse(defaultTimeFormat, string(entry.GetValue()))
		if err != nil {
			log.Error("Failed parsing time for ipns record EOL")
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
