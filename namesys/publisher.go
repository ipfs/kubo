package namesys

import (
	"bytes"
	"context"
	"fmt"
	"time"

	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	ft "github.com/ipfs/go-ipfs/unixfs"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	routing "gx/ipfs/QmTiWLZ6Fo5j4KcTVutZJ5KWRRJrbxzmxA4td8NfEdrPh7/go-libp2p-routing"
	dshelp "gx/ipfs/QmTmqJGRQfuH8eKWD1FjThwPRipt1QhqJQNZ8MpzmfAAxo/go-ipfs-ds-help"
	dhtpb "gx/ipfs/QmUpttFinNDmNPgFwKN8sZK6BUtBmA68Y4KdSBDXa8t9sJ/go-libp2p-record/pb"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

const PublishPutValTimeout = time.Minute
const DefaultRecordTTL = 24 * time.Hour

// ipnsPublisher is capable of publishing and resolving names to the IPFS
// routing system.
type ipnsPublisher struct {
	routing routing.ValueStore
	ds      ds.Datastore
}

// NewRoutingPublisher constructs a publisher for the IPFS Routing name system.
func NewRoutingPublisher(route routing.ValueStore, ds ds.Datastore) *ipnsPublisher {
	if ds == nil {
		panic("nil datastore")
	}
	return &ipnsPublisher{routing: route, ds: ds}
}

// Publish implements Publisher. Accepts a keypair and a value,
// and publishes it out to the routing system
func (p *ipnsPublisher) Publish(ctx context.Context, k ci.PrivKey, value path.Path) error {
	log.Debugf("Publish %s", value)
	return p.PublishWithEOL(ctx, k, value, time.Now().Add(DefaultRecordTTL))
}

// PublishWithEOL is a temporary stand in for the ipns records implementation
// see here for more details: https://github.com/ipfs/specs/tree/master/records
func (p *ipnsPublisher) PublishWithEOL(ctx context.Context, k ci.PrivKey, value path.Path, eol time.Time) error {

	id, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return err
	}

	_, ipnskey := IpnsKeysForID(id)

	// get previous records sequence number
	seqnum, err := p.getPreviousSeqNo(ctx, ipnskey)
	if err != nil {
		return err
	}

	// increment it
	seqnum++

	return PutRecordToRouting(ctx, k, value, seqnum, eol, p.routing, id)
}

func (p *ipnsPublisher) getPreviousSeqNo(ctx context.Context, ipnskey string) (uint64, error) {
	prevrec, err := p.ds.Get(dshelp.NewKeyFromBinary([]byte(ipnskey)))
	if err != nil && err != ds.ErrNotFound {
		// None found, lets start at zero!
		return 0, err
	}
	var val []byte
	if err == nil {
		prbytes, ok := prevrec.([]byte)
		if !ok {
			return 0, fmt.Errorf("unexpected type returned from datastore: %#v", prevrec)
		}
		dhtrec := new(dhtpb.Record)
		err := proto.Unmarshal(prbytes, dhtrec)
		if err != nil {
			return 0, err
		}

		val = dhtrec.GetValue()
	} else {
		// try and check the dht for a record
		ctx, cancel := context.WithTimeout(ctx, time.Second*30)
		defer cancel()

		rv, err := p.routing.GetValue(ctx, ipnskey)
		if err != nil {
			// no such record found, start at zero!
			return 0, nil
		}

		val = rv
	}

	e := new(pb.IpnsEntry)
	err = proto.Unmarshal(val, e)
	if err != nil {
		return 0, err
	}

	return e.GetSequence(), nil
}

// setting the TTL on published records is an experimental feature.
// as such, i'm using the context to wire it through to avoid changing too
// much code along the way.
func checkCtxTTL(ctx context.Context) (time.Duration, bool) {
	v := ctx.Value("ipns-publish-ttl")
	if v == nil {
		return 0, false
	}

	d, ok := v.(time.Duration)
	return d, ok
}

func PutRecordToRouting(ctx context.Context, k ci.PrivKey, value path.Path, seqnum uint64, eol time.Time, r routing.ValueStore, id peer.ID) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	namekey, ipnskey := IpnsKeysForID(id)
	entry, err := CreateRoutingEntryData(k, value, seqnum, eol)
	if err != nil {
		return err
	}

	ttl, ok := checkCtxTTL(ctx)
	if ok {
		entry.Ttl = proto.Uint64(uint64(ttl.Nanoseconds()))
	}

	errs := make(chan error, 2) // At most two errors (IPNS, and public key)

	// Attempt to extract the public key from the ID
	extractedPublicKey := id.ExtractPublicKey()

	go func() {
		errs <- PublishEntry(ctx, r, ipnskey, entry)
	}()

	// Publish the public key if a public key cannot be extracted from the ID
	if extractedPublicKey == nil {
		go func() {
			errs <- PublishPublicKey(ctx, r, namekey, k.GetPublic())
		}()

		if err := waitOnErrChan(ctx, errs); err != nil {
			return err
		}
	}

	return waitOnErrChan(ctx, errs)
}

func waitOnErrChan(ctx context.Context, errs chan error) error {
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func PublishPublicKey(ctx context.Context, r routing.ValueStore, k string, pubk ci.PubKey) error {
	log.Debugf("Storing pubkey at: %s", k)
	pkbytes, err := pubk.Bytes()
	if err != nil {
		return err
	}

	// Store associated public key
	timectx, cancel := context.WithTimeout(ctx, PublishPutValTimeout)
	defer cancel()
	return r.PutValue(timectx, k, pkbytes)
}

func PublishEntry(ctx context.Context, r routing.ValueStore, ipnskey string, rec *pb.IpnsEntry) error {
	timectx, cancel := context.WithTimeout(ctx, PublishPutValTimeout)
	defer cancel()

	data, err := proto.Marshal(rec)
	if err != nil {
		return err
	}

	log.Debugf("Storing ipns entry at: %s", ipnskey)
	// Store ipns entry at "/ipns/"+h(pubkey)
	return r.PutValue(timectx, ipnskey, data)
}

func CreateRoutingEntryData(pk ci.PrivKey, val path.Path, seq uint64, eol time.Time) (*pb.IpnsEntry, error) {
	entry := new(pb.IpnsEntry)

	entry.Value = []byte(val)
	typ := pb.IpnsEntry_EOL
	entry.ValidityType = &typ
	entry.Sequence = proto.Uint64(seq)
	entry.Validity = []byte(u.FormatRFC3339(eol))

	sig, err := pk.Sign(ipnsEntryDataForSig(entry))
	if err != nil {
		return nil, err
	}
	entry.Signature = sig
	return entry, nil
}

func ipnsEntryDataForSig(e *pb.IpnsEntry) []byte {
	return bytes.Join([][]byte{
		e.Value,
		e.Validity,
		[]byte(fmt.Sprint(e.GetValidityType())),
	},
		[]byte{})
}

// InitializeKeyspace sets the ipns record for the given key to
// point to an empty directory.
// TODO: this doesnt feel like it belongs here
func InitializeKeyspace(ctx context.Context, pub Publisher, pins pin.Pinner, key ci.PrivKey) error {
	emptyDir := ft.EmptyDirNode()

	// pin recursively because this might already be pinned
	// and doing a direct pin would throw an error in that case
	err := pins.Pin(ctx, emptyDir, true)
	if err != nil {
		return err
	}

	err = pins.Flush()
	if err != nil {
		return err
	}

	return pub.Publish(ctx, key, path.FromCid(emptyDir.Cid()))
}

func IpnsKeysForID(id peer.ID) (name, ipns string) {
	namekey := "/pk/" + string(id)
	ipnskey := "/ipns/" + string(id)

	return namekey, ipnskey
}
