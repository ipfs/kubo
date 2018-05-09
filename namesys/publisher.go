package namesys

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	ft "github.com/ipfs/go-ipfs/unixfs"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	routing "gx/ipfs/QmUHRKTeaoASDvDj7cTAXsmjAY7KQ13ErtzkQHZQq6uFUz/go-libp2p-routing"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	peer "gx/ipfs/QmcJukH2sAFjY3HdBKq35WDzWoL3UUu2gt9wdfqZTUyM74/go-libp2p-peer"
	ci "gx/ipfs/Qme1knMqwt1hKZbc1BmQFmnm9f36nyQGwXxPGVpVJ9rMK5/go-libp2p-crypto"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
	dsquery "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore/query"
	base32 "gx/ipfs/QmfVj3x4D6Jkq9SEoi5n2NmoUomLwoeiwnYz2KQa15wRw6/base32"
)

const ipnsPrefix = "/ipns/"

const PublishPutValTimeout = time.Minute
const DefaultRecordTTL = 24 * time.Hour

// IpnsPublisher is capable of publishing and resolving names to the IPFS
// routing system.
type IpnsPublisher struct {
	routing routing.ValueStore
	ds      ds.Datastore

	// Used to ensure we assign IPNS records *sequential* sequence numbers.
	mu sync.Mutex
}

// NewIpnsPublisher constructs a publisher for the IPFS Routing name system.
func NewIpnsPublisher(route routing.ValueStore, ds ds.Datastore) *IpnsPublisher {
	if ds == nil {
		panic("nil datastore")
	}
	return &IpnsPublisher{routing: route, ds: ds}
}

// Publish implements Publisher. Accepts a keypair and a value,
// and publishes it out to the routing system
func (p *IpnsPublisher) Publish(ctx context.Context, k ci.PrivKey, value path.Path) error {
	log.Debugf("Publish %s", value)
	return p.PublishWithEOL(ctx, k, value, time.Now().Add(DefaultRecordTTL))
}

func IpnsDsKey(id peer.ID) ds.Key {
	return ds.NewKey("/ipns/" + base32.RawStdEncoding.EncodeToString([]byte(id)))
}

// PublishedNames returns the latest IPNS records published by this node and
// their expiration times.
//
// This method will not search the routing system for records published by other
// nodes.
func (p *IpnsPublisher) ListPublished(ctx context.Context) (map[peer.ID]*pb.IpnsEntry, error) {
	query, err := p.ds.Query(dsquery.Query{
		Prefix: ipnsPrefix,
	})
	if err != nil {
		return nil, err
	}
	defer query.Close()

	records := make(map[peer.ID]*pb.IpnsEntry)
	for {
		select {
		case result, ok := <-query.Next():
			if !ok {
				return records, nil
			}
			if result.Error != nil {
				return nil, result.Error
			}
			value, ok := result.Value.([]byte)
			if !ok {
				log.Error("found ipns record that we couldn't convert to a value")
				continue
			}
			e := new(pb.IpnsEntry)
			if err := proto.Unmarshal(value, e); err != nil {
				// Might as well return what we can.
				log.Error("found an invalid IPNS entry:", err)
				continue
			}
			if !strings.HasPrefix(result.Key, ipnsPrefix) {
				log.Errorf("datastore query for keys with prefix %s returned a key: %s", ipnsPrefix, result.Key)
				continue
			}
			k := result.Key[len(ipnsPrefix):]
			pid, err := base32.RawStdEncoding.DecodeString(k)
			if err != nil {
				log.Errorf("ipns ds key invalid: %s", result.Key)
				continue
			}
			records[peer.ID(pid)] = e
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// GetPublished returns the record this node has published corresponding to the
// given peer ID.
//
// If `checkRouting` is true and we have no existing record, this method will
// check the routing system for any existing records.
func (p *IpnsPublisher) GetPublished(ctx context.Context, id peer.ID, checkRouting bool) (*pb.IpnsEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	dsVal, err := p.ds.Get(IpnsDsKey(id))
	var value []byte
	switch err {
	case nil:
		var ok bool
		value, ok = dsVal.([]byte)
		if !ok {
			return nil, fmt.Errorf("found ipns record that we couldn't convert to a value")
		}
	case ds.ErrNotFound:
		if !checkRouting {
			return nil, nil
		}
		_, ipnskey := IpnsKeysForID(id)
		value, err = p.routing.GetValue(ctx, ipnskey)
		if err != nil {
			// Not found or other network issue. Can't really do
			// anything about this case.
			return nil, nil
		}
	default:
		return nil, err
	}
	e := new(pb.IpnsEntry)
	if err := proto.Unmarshal(value, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (p *IpnsPublisher) updateRecord(ctx context.Context, k ci.PrivKey, value path.Path, eol time.Time) (*pb.IpnsEntry, error) {
	id, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// get previous records sequence number
	rec, err := p.GetPublished(ctx, id, true)
	if err != nil {
		return nil, err
	}

	seqno := rec.GetSequence() // returns 0 if rec is nil
	if rec != nil && value != path.Path(rec.GetValue()) {
		// Don't bother incrementing the sequence number unless the
		// value changes.
		seqno++
	}

	// Create record
	entry, err := CreateRoutingEntryData(k, value, seqno, eol)
	if err != nil {
		return nil, err
	}

	// Set the TTL
	// TODO: Make this less hacky.
	ttl, ok := checkCtxTTL(ctx)
	if ok {
		entry.Ttl = proto.Uint64(uint64(ttl.Nanoseconds()))
	}

	data, err := proto.Marshal(entry)
	if err != nil {
		return nil, err
	}

	// Put the new record.
	if err := p.ds.Put(IpnsDsKey(id), data); err != nil {
		return nil, err
	}
	return entry, nil
}

// PublishWithEOL is a temporary stand in for the ipns records implementation
// see here for more details: https://github.com/ipfs/specs/tree/master/records
func (p *IpnsPublisher) PublishWithEOL(ctx context.Context, k ci.PrivKey, value path.Path, eol time.Time) error {
	record, err := p.updateRecord(ctx, k, value, eol)
	if err != nil {
		return err
	}

	return PutRecordToRouting(ctx, p.routing, k.GetPublic(), record)
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

func PutRecordToRouting(ctx context.Context, r routing.ValueStore, k ci.PubKey, entry *pb.IpnsEntry) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, 2) // At most two errors (IPNS, and public key)

	id, err := peer.IDFromPublicKey(k)
	if err != nil {
		return err
	}

	// Attempt to extract the public key from the ID
	extractedPublicKey, err := id.ExtractPublicKey()
	if err != nil {
		return err
	}

	namekey, ipnskey := IpnsKeysForID(id)

	go func() {
		errs <- PublishEntry(ctx, r, ipnskey, entry)
	}()

	// Publish the public key if a public key cannot be extracted from the ID
	if extractedPublicKey == nil {
		go func() {
			errs <- PublishPublicKey(ctx, r, namekey, k)
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
