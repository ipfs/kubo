package namesys

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	proto "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	dag "github.com/ipfs/go-ipfs/merkledag"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	routing "github.com/ipfs/go-ipfs/routing"
	record "github.com/ipfs/go-ipfs/routing/record"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"
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
func (p *ipnsPublisher) Publish(ctx context.Context, k ci.PrivKey, value path.Path) error {
	log.Debugf("Publish %s", value)

	pubkey := k.GetPublic()
	pkbytes, err := pubkey.Bytes()
	if err != nil {
		return err
	}

	nameb := u.Hash(pkbytes)
	namekey := key.Key("/pk/" + string(nameb))
	ipnskey := key.Key("/ipns/" + string(nameb))

	// get previous records sequence number, and add one to it
	var seqnum uint64
	prevrec, err := p.routing.GetValues(ctx, ipnskey, 0)
	if err == nil {
		e := new(pb.IpnsEntry)
		err := proto.Unmarshal(prevrec[0].Val, e)
		if err != nil {
			return err
		}

		seqnum = e.GetSequence() + 1
	} else if err != ds.ErrNotFound {
		return err
	}

	data, err := createRoutingEntryData(k, value, seqnum)
	if err != nil {
		return err
	}

	log.Debugf("Storing pubkey at: %s", namekey)
	// Store associated public key
	timectx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second*10))
	defer cancel()
	err = p.routing.PutValue(timectx, namekey, pkbytes)
	if err != nil {
		return err
	}

	log.Debugf("Storing ipns entry at: %s", ipnskey)
	// Store ipns entry at "/ipns/"+b58(h(pubkey))
	timectx, cancel = context.WithDeadline(ctx, time.Now().Add(time.Second*10))
	defer cancel()
	if err := p.routing.PutValue(timectx, ipnskey, data); err != nil {
		return err
	}

	return nil
}

func createRoutingEntryData(pk ci.PrivKey, val path.Path, seq uint64) ([]byte, error) {
	entry := new(pb.IpnsEntry)

	entry.Value = []byte(val)
	typ := pb.IpnsEntry_EOL
	entry.ValidityType = &typ
	entry.Sequence = proto.Uint64(seq)
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

var IpnsRecordValidator = &record.ValidChecker{
	Func: ValidateIpnsRecord,
	Sign: true,
}

func IpnsSelectorFunc(k key.Key, vals [][]byte) (int, error) {
	var recs []*pb.IpnsEntry
	for _, v := range vals {
		e := new(pb.IpnsEntry)
		err := proto.Unmarshal(v, e)
		if err == nil {
			recs = append(recs, e)
		} else {
			recs = append(recs, nil)
		}
	}

	var best_seq uint64
	best_i := -1

	for i, r := range recs {
		if r == nil {
			continue
		}
		if best_i == -1 || r.GetSequence() > best_seq {
			best_seq = r.GetSequence()
			best_i = i
		} else if r.GetSequence() == best_seq {
			rt, err := u.ParseRFC3339(string(r.GetValidity()))
			if err != nil {
				continue
			}

			bestt, err := u.ParseRFC3339(string(recs[best_i].GetValidity()))
			if err != nil {
				continue
			}

			if rt.After(bestt) {
				best_seq = r.GetSequence()
				best_i = i
			}
		}
	}
	if best_i == -1 {
		return 0, errors.New("no usable records in given set")
	}

	return best_i, nil
}

// ValidateIpnsRecord implements ValidatorFunc and verifies that the
// given 'val' is an IpnsEntry and that that entry is valid.
func ValidateIpnsRecord(k key.Key, val []byte) error {
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

// InitializeKeyspace sets the ipns record for the given key to
// point to an empty directory.
// TODO: this doesnt feel like it belongs here
func InitializeKeyspace(ctx context.Context, ds dag.DAGService, pub Publisher, pins pin.Pinner, key ci.PrivKey) error {
	emptyDir := &dag.Node{Data: ft.FolderPBData()}
	nodek, err := ds.Add(emptyDir)
	if err != nil {
		return err
	}

	// pin recursively because this might already be pinned
	// and doing a direct pin would throw an error in that case
	err = pins.Pin(ctx, emptyDir, true)
	if err != nil {
		return err
	}

	err = pins.Flush()
	if err != nil {
		return err
	}

	err = pub.Publish(ctx, key, path.FromKey(nodek))
	if err != nil {
		return err
	}

	return nil
}
