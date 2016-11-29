package namesys

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	dag "github.com/ipfs/go-ipfs/merkledag"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
	ft "github.com/ipfs/go-ipfs/unixfs"

	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
	routing "gx/ipfs/QmbkGVaN9W6RYJK4Ws5FvMKXKDqdRQ5snhtaa92qP6L8eU/go-libp2p-routing"
	record "gx/ipfs/QmdM4ohF7cr4MvAECVeD3hRA3HtZrk1ngaek4n8ojVT87h/go-libp2p-record"
	dhtpb "gx/ipfs/QmdM4ohF7cr4MvAECVeD3hRA3HtZrk1ngaek4n8ojVT87h/go-libp2p-record/pb"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	ci "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"
)

var errInvalidEntry = errors.New("invalid previous entry")

// ErrExpiredRecord should be returned when an ipns record is
// invalid due to being too old
var ErrExpiredRecord = errors.New("expired record")

// ErrUnrecognizedValidity is returned when an IpnsRecord has an
// unknown validity type.
var ErrUnrecognizedValidity = errors.New("unrecognized validity type")

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
func (p *ipnsPublisher) PublishWithEOL(ctx context.Context, pk ci.PrivKey, path path.Path, eol time.Time) error {
	id, err := peer.IDFromPrivateKey(pk)
	if err != nil {
		return err
	}

	// get previous records sequence number
	rec, err := p.tryLocalThenRemote(ctx, IpnsKeyForID(id))
	if err != nil {
		return err
	}

	_, seq, err := p.parseIpnsRecord(rec)
	if err != nil {
		return err
	}

	// increment it
	seq++

	return PutRecordToRouting(ctx, pk, path, seq, eol, p.routing, id)
}

func (p *ipnsPublisher) RePublish(ctx context.Context, pk ci.PrivKey, eol time.Time) error {
	id, err := peer.IDFromPrivateKey(pk)
	if err != nil {
		return err
	}

	record, err := p.getLocal(ctx, IpnsKeyForID(id))
	if err == ds.ErrNotFound {
		// not found means we don't have a previously published entry
		return nil
	}
	if err != nil {
		return err
	}

	path, seq, err := p.parseIpnsRecord(record)
	if err != nil {
		return err
	}

	// update record with same sequence number
	return PutRecordToRouting(ctx, pk, path, seq, eol, p.routing, id)
}

func (p *ipnsPublisher) Upload(ctx context.Context, pk ci.PubKey, record []byte) (id peer.ID, oldSeq uint64, newSeq uint64, newPath path.Path, err error) {
	id, err = peer.IDFromPublicKey(pk)
	if err != nil {
		return
	}

	// get previous records sequence number
	oldRec, err := p.tryLocalThenRemote(ctx, IpnsKeyForID(id))
	if err != nil {
		return
	}

	_, oldSeq, err = p.parseIpnsRecord(oldRec)
	if err != nil {
		return
	}

	entry := new(pb.IpnsEntry)
	err = proto.Unmarshal(record, entry)
	if err != nil {
		return
	}

	if ok, err1 := pk.Verify(ipnsEntryDataForSig(entry), entry.GetSignature()); err1 != nil || !ok {
		err = fmt.Errorf("The IPNS record is not signed by %s", id.Pretty())
		return
	}
	if entry.Sequence == nil {
		err = errors.New("The IPNS record must have a sequence number, but none were provided")
		return
	}

	newSeq = *entry.Sequence
	if newSeq <= oldSeq {
		err = fmt.Errorf("There is already a newer IPNS record with sequence number: %d", oldSeq)
		return
	}

	newPath, err = path.ParsePath(string(entry.Value))
	if err != nil {
		return
	}

	err = publishEntry(ctx, p.routing, pk, entry)
	return
}

func (p *ipnsPublisher) tryLocalThenRemote(ctx context.Context, dhtKey string) ([]byte, error) {
	dhtValue, err := p.getLocal(ctx, dhtKey)
	if err == ds.ErrNotFound {
		dhtValue, err = p.getRemote(ctx, dhtKey)
		if err != nil {
			return nil, nil
		}
	}
	return dhtValue, err
}

func (p *ipnsPublisher) getRemote(ctx context.Context, dhtKey string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	return p.routing.GetValue(ctx, dhtKey)
}

func (p *ipnsPublisher) getLocal(ctx context.Context, dhtKey string) ([]byte, error) {
	maybeDsVal, err := p.ds.Get(dshelp.NewKeyFromBinary([]byte(dhtKey)))
	if err != nil {
		return nil, err
	}

	dsVal, ok := maybeDsVal.([]byte)
	if !ok {
		return nil, errInvalidEntry
	}

	dhtRec := new(dhtpb.Record)
	err = proto.Unmarshal(dsVal, dhtRec)
	if err != nil {
		return nil, err
	}

	return dhtRec.GetValue(), nil
}

func (p *ipnsPublisher) parseIpnsRecord(record []byte) (path.Path, uint64, error) {
	if record == nil {
		return "", 0, nil
	}

	// extract published data from record
	ipnsEntry := new(pb.IpnsEntry)
	err := proto.Unmarshal(record, ipnsEntry)
	if err != nil {
		return "", 0, err
	}

	return path.Path(ipnsEntry.Value), ipnsEntry.GetSequence(), nil
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

func PutRecordToRouting(ctx context.Context, sk ci.PrivKey, value path.Path, seqnum uint64, eol time.Time, r routing.ValueStore, id peer.ID) error {
	entry, err := createEntry(ctx, sk, value, seqnum, eol)
	if err != nil {
		return err
	}

	pk := sk.GetPublic()

	return publishEntry(ctx, r, pk, entry)
}

func CreateEntry(sk ci.PrivKey, value path.Path, seqnum uint64, eol time.Time, ttl time.Duration) ([]byte, error) {
	ctx := context.WithValue(context.Background(), "ipns-publish-ttl", ttl)
	entry, err := createEntry(ctx, sk, value, seqnum, eol)
	if err != nil {
		return nil, err
	}

	result, err := proto.Marshal(entry)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func createEntry(ctx context.Context, sk ci.PrivKey, value path.Path, seqnum uint64, eol time.Time) (*pb.IpnsEntry, error) {
	entry, err := CreateRoutingEntryData(sk, value, seqnum, eol)
	if err != nil {
		return nil, err
	}

	ttl, ok := checkCtxTTL(ctx)
	if ok {
		entry.Ttl = proto.Uint64(uint64(ttl.Nanoseconds()))
	}

	return entry, nil
}

func publishEntry(ctx context.Context, r routing.ValueStore, pk ci.PubKey, entry *pb.IpnsEntry) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, 2)

	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return err
	}

	go func() {
		errs <- PublishEntry(ctx, r, id, entry)
	}()

	go func() {
		errs <- PublishPublicKey(ctx, r, id, pk)
	}()

	err = waitOnErrChan(ctx, errs)
	if err != nil {
		return err
	}

	err = waitOnErrChan(ctx, errs)
	if err != nil {
		return err
	}

	return nil
}

func waitOnErrChan(ctx context.Context, errs chan error) error {
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func PublishPublicKey(ctx context.Context, r routing.ValueStore, id peer.ID, pk ci.PubKey) error {
	// Store associated public key
	timectx, cancel := context.WithTimeout(ctx, PublishPutValTimeout)
	defer cancel()

	dhtValue, err := pk.Bytes()
	if err != nil {
		return err
	}

	dhtKey := routing.KeyForPublicKey(id)
	log.Debugf("Storing pubkey at: %s", dhtKey)
	err = r.PutValue(timectx, dhtKey, dhtValue)
	if err != nil {
		return err
	}

	return nil
}

func PublishEntry(ctx context.Context, r routing.ValueStore, id peer.ID, rec *pb.IpnsEntry) error {
	timectx, cancel := context.WithTimeout(ctx, PublishPutValTimeout)
	defer cancel()

	dhtValue, err := proto.Marshal(rec)
	if err != nil {
		return err
	}

	dhtKey := IpnsKeyForID(id)
	log.Debugf("Storing ipns entry at: %s", dhtKey)
	// Store ipns entry at "/ipns/"+b58(h(pubkey))
	if err := r.PutValue(timectx, dhtKey, dhtValue); err != nil {
		return err
	}

	return nil
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

var IpnsRecordValidator = &record.ValidChecker{
	Func: ValidateIpnsRecord,
	Sign: true,
}

func IpnsSelectorFunc(k string, vals [][]byte) (int, error) {
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

	return selectRecord(recs, vals)
}

func selectRecord(recs []*pb.IpnsEntry, vals [][]byte) (int, error) {
	var best_seq uint64
	best_i := -1

	for i, r := range recs {
		if r == nil || r.GetSequence() < best_seq {
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
				best_i = i
			} else if rt == bestt {
				if bytes.Compare(vals[i], vals[best_i]) > 0 {
					best_i = i
				}
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
func ValidateIpnsRecord(k string, val []byte) error {
	entry := new(pb.IpnsEntry)
	err := proto.Unmarshal(val, entry)
	if err != nil {
		return err
	}
	switch entry.GetValidityType() {
	case pb.IpnsEntry_EOL:
		t, err := u.ParseRFC3339(string(entry.GetValidity()))
		if err != nil {
			log.Debug("failed parsing time for ipns record EOL")
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
// TODO: compare to github.com/ipfs/go-ipfs/fuse/ipns/common.go
func InitializeKeyspace(ctx context.Context, ds dag.DAGService, pub Publisher, pins pin.Pinner, key ci.PrivKey) error {
	emptyDir := ft.EmptyDirNode()
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

	err = pub.Publish(ctx, key, path.FromCid(nodek))
	if err != nil {
		return err
	}

	return nil
}

func IpnsKeyForID(id peer.ID) string {
	return "/ipns/" + string(id)
}
