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
	ds "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	routing "gx/ipfs/QmTiWLZ6Fo5j4KcTVutZJ5KWRRJrbxzmxA4td8NfEdrPh7/go-libp2p-routing"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

const PublishPutValTimeout = time.Minute
const DefaultRecordTTL = 24 * time.Hour

// ErrMsgNoRecordsGiven is the error message returned from
// Selector.BestRecord() when there are no records passed to it
// https://github.com/libp2p/go-libp2p-record/blob/master/selection.go#L15
// Unfortunately it is returned by routing.ValueStore.GetValue() instead of
// routing.ErrNotFound so this is the only way we can test for it
// TODO: Fix this
var ErrMsgNoRecordsGiven = "no records given"

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
	_, err := dhtPublishWithEOL(ctx, p.ds, p.routing, k, value, eol)
	return err
}

func dhtPublishWithEOL(ctx context.Context, dstore ds.Datastore, vstore routing.ValueStore, k ci.PrivKey, value path.Path, eol time.Time) (*pb.IpnsEntry, error) {
	id, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return nil, err
	}

	seqnum, err := getNextSeqNum(ctx, vstore, id, value)
	if err != nil {
		return nil, err
	}

	return PublishRecordWithSeqNum(ctx, dstore, vstore, k, id, value, seqnum, eol)
}

// PublishRecordWithSeqNum publishes a record with the given sequence number to the DHT
func PublishRecordWithSeqNum(ctx context.Context, dstore ds.Datastore, vstore routing.ValueStore, k ci.PrivKey, id peer.ID, value path.Path, seqnum uint64, eol time.Time) (*pb.IpnsEntry, error) {
	// Publish the record
	entry, err := PutRecordToRouting(ctx, k, value, seqnum, eol, vstore, id)
	if err != nil {
		return nil, err
	}

	// Save published sequence number and value in the republisher cache
	err = RepubCachePut(dstore, id, seqnum, value)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func getNextSeqNum(ctx context.Context, vstore routing.ValueStore, id peer.ID, value path.Path) (uint64, error) {
	log.Debugf("GetNextSeqNum %s", id)

	// Get the existing entry for this key from the DHT
	seqnum := uint64(0)
	_, ipnskey := IpnsKeysForID(id)
	existing, err := GetExistingEntry(ctx, vstore, ipnskey)
	if err != nil && err != routing.ErrNotFound {
		log.Debugf("Error getting existing entry for %s: %v", id, err)
		return seqnum, err
	}

	// If there is an existing entry, and the value is different, increment
	// the sequence number
	if existing != nil {
		seqnum = existing.GetSequence()
		log.Debugf("Found existing entry for %s with seqnum %d", id, seqnum)
		if bytes.Compare(existing.GetValue(), []byte(value)) != 0 {
			seqnum++
			log.Debugf("New seqnum for %s: %d", id, seqnum)
		}
	} else {
		log.Debugf("Did not find existing entry for %s", id)
	}

	return seqnum, nil
}

// GetExistingEntry gets the entry with the given ipns key from the DHT
func GetExistingEntry(ctx context.Context, vstore routing.ValueStore, ipnskey string) (*pb.IpnsEntry, error) {
	// try and check the dht for a record
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	val, err := vstore.GetValue(ctx, ipnskey)
	if err != nil {
		// TODO: Fix this string comparison
		if err == ds.ErrNotFound || err.Error() == ErrMsgNoRecordsGiven {
			return nil, routing.ErrNotFound
		}
		return nil, err
	}

	e := new(pb.IpnsEntry)
	err = proto.Unmarshal(val, e)
	if err != nil {
		return nil, err
	}

	return e, nil
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

// PutRecordToRouting puts the IPNS entry and its associated public key to the DHT
func PutRecordToRouting(ctx context.Context, k ci.PrivKey, value path.Path, seqnum uint64, eol time.Time, r routing.ValueStore, id peer.ID) (*pb.IpnsEntry, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	namekey, ipnskey := IpnsKeysForID(id)
	entry, err := CreateRoutingEntryData(k, value, seqnum, eol)
	if err != nil {
		return nil, err
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
			return nil, err
		}
	}

	if err := waitOnErrChan(ctx, errs); err != nil {
		return nil, err
	}

	return entry, nil
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
