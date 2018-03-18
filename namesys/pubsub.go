package namesys

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	p2phost "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	floodsub "gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
	routing "gx/ipfs/QmTiWLZ6Fo5j4KcTVutZJ5KWRRJrbxzmxA4td8NfEdrPh7/go-libp2p-routing"
	dshelp "gx/ipfs/QmTmqJGRQfuH8eKWD1FjThwPRipt1QhqJQNZ8MpzmfAAxo/go-ipfs-ds-help"
	record "gx/ipfs/QmUpttFinNDmNPgFwKN8sZK6BUtBmA68Y4KdSBDXa8t9sJ/go-libp2p-record"
	dhtpb "gx/ipfs/QmUpttFinNDmNPgFwKN8sZK6BUtBmA68Y4KdSBDXa8t9sJ/go-libp2p-record/pb"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	dssync "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore/sync"
	pstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// PubsubPublisher is a publisher that distributes IPNS records through pubsub
type PubsubPublisher struct {
	ctx  context.Context
	ds   ds.Datastore
	host p2phost.Host
	cr   routing.ContentRouting
	ps   *floodsub.PubSub

	mx   sync.Mutex
	subs map[string]struct{}
}

// PubsubResolver is a resolver that receives IPNS records through pubsub
type PubsubResolver struct {
	ctx  context.Context
	ds   ds.Datastore
	host p2phost.Host
	cr   routing.ContentRouting
	pkf  routing.PubKeyFetcher
	ps   *floodsub.PubSub

	mx   sync.Mutex
	subs map[string]*floodsub.Subscription
}

// NewPubsubPublisher constructs a new Publisher that publishes IPNS records through pubsub.
// The constructor interface is complicated by the need to bootstrap the pubsub topic.
// This could be greatly simplified if the pubsub implementation handled bootstrap itself
func NewPubsubPublisher(ctx context.Context, host p2phost.Host, ds ds.Datastore, cr routing.ContentRouting, ps *floodsub.PubSub) *PubsubPublisher {
	return &PubsubPublisher{
		ctx:  ctx,
		ds:   ds,
		host: host, // needed for pubsub bootstrap
		cr:   cr,   // needed for pubsub bootstrap
		ps:   ps,
		subs: make(map[string]struct{}),
	}
}

// NewPubsubResolver constructs a new Resolver that resolves IPNS records through pubsub.
// same as above for pubsub bootstrap dependencies
func NewPubsubResolver(ctx context.Context, host p2phost.Host, cr routing.ContentRouting, pkf routing.PubKeyFetcher, ps *floodsub.PubSub) *PubsubResolver {
	return &PubsubResolver{
		ctx:  ctx,
		ds:   dssync.MutexWrap(ds.NewMapDatastore()),
		host: host, // needed for pubsub bootstrap
		cr:   cr,   // needed for pubsub bootstrap
		pkf:  pkf,
		ps:   ps,
		subs: make(map[string]*floodsub.Subscription),
	}
}

// Publish publishes an IPNS record through pubsub with default TTL
func (p *PubsubPublisher) Publish(ctx context.Context, k ci.PrivKey, value path.Path) error {
	return p.PublishWithEOL(ctx, k, value, time.Now().Add(DefaultRecordTTL))
}

// PublishWithEOL publishes an IPNS record through pubsub
func (p *PubsubPublisher) PublishWithEOL(ctx context.Context, k ci.PrivKey, value path.Path, eol time.Time) error {
	id, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return err
	}

	_, ipnskey := IpnsKeysForID(id)

	seqno, err := p.getPreviousSeqNo(ctx, ipnskey)
	if err != nil {
		return err
	}

	seqno++

	return p.publishRecord(ctx, k, value, seqno, eol, ipnskey, id)
}

func (p *PubsubPublisher) getPreviousSeqNo(ctx context.Context, ipnskey string) (uint64, error) {
	// the datastore is shared with the routing publisher to properly increment and persist
	// ipns record sequence numbers.
	prevrec, err := p.ds.Get(dshelp.NewKeyFromBinary([]byte(ipnskey)))
	if err != nil {
		if err == ds.ErrNotFound {
			// None found, lets start at zero!
			return 0, nil
		}
		return 0, err
	}

	prbytes, ok := prevrec.([]byte)
	if !ok {
		return 0, fmt.Errorf("unexpected type returned from datastore: %#v", prevrec)
	}

	var dsrec dhtpb.Record
	err = proto.Unmarshal(prbytes, &dsrec)
	if err != nil {
		return 0, err
	}

	var entry pb.IpnsEntry
	err = proto.Unmarshal(dsrec.GetValue(), &entry)
	if err != nil {
		return 0, err
	}

	return entry.GetSequence(), nil
}

func (p *PubsubPublisher) publishRecord(ctx context.Context, k ci.PrivKey, value path.Path, seqno uint64, eol time.Time, ipnskey string, ID peer.ID) error {
	entry, err := CreateRoutingEntryData(k, value, seqno, eol)
	if err != nil {
		return err
	}

	data, err := proto.Marshal(entry)
	if err != nil {
		return err
	}

	// the datastore is shared with the routing publisher to properly increment and persist
	// ipns record sequence numbers; so we need to Record our new entry in the datastore
	dsrec, err := record.MakePutRecord(k, ipnskey, data, true)
	if err != nil {
		return err
	}

	dsdata, err := proto.Marshal(dsrec)
	if err != nil {
		return err
	}

	err = p.ds.Put(dshelp.NewKeyFromBinary([]byte(ipnskey)), dsdata)
	if err != nil {
		return err
	}

	// now we publish, but we also need to bootstrap pubsub for our messages to propagate
	topic := "/ipns/" + ID.Pretty()

	p.mx.Lock()
	_, ok := p.subs[topic]

	if !ok {
		p.subs[topic] = struct{}{}
		p.mx.Unlock()

		bootstrapPubsub(p.ctx, p.cr, p.host, topic)
	} else {
		p.mx.Unlock()
	}

	log.Debugf("PubsubPublish: publish IPNS record for %s (%d)", topic, seqno)
	return p.ps.Publish(topic, data)
}

// Resolve resolves a name through pubsub and default depth limit
func (r *PubsubResolver) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	return resolve(ctx, r, name, opts.ProcessOpts(options), "/ipns/")
}

func (r *PubsubResolver) resolveOnce(ctx context.Context, name string, options *opts.ResolveOpts) (path.Path, error) {
	log.Debugf("PubsubResolve: resolve '%s'", name)

	// retrieve the public key once (for verifying messages)
	xname := strings.TrimPrefix(name, "/ipns/")
	hash, err := mh.FromB58String(xname)
	if err != nil {
		log.Warningf("PubsubResolve: bad input hash: [%s]", xname)
		return "", err
	}

	id := peer.ID(hash)
	if r.host.Peerstore().PrivKey(id) != nil {
		return "", errors.New("cannot resolve own name through pubsub")
	}

	pubk := id.ExtractPublicKey()
	if pubk == nil {
		pubk, err = r.pkf.GetPublicKey(ctx, id)
		if err != nil {
			log.Warningf("PubsubResolve: error fetching public key: %s [%s]", err.Error(), xname)
			return "", err
		}
	}

	// the topic is /ipns/Qmhash
	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}

	r.mx.Lock()
	// see if we already have a pubsub subscription; if not, subscribe
	sub, ok := r.subs[name]
	if !ok {
		sub, err = r.ps.Subscribe(name)
		if err != nil {
			r.mx.Unlock()
			return "", err
		}

		log.Debugf("PubsubResolve: subscribed to %s", name)

		r.subs[name] = sub

		ctx, cancel := context.WithCancel(r.ctx)
		go r.handleSubscription(sub, name, pubk, cancel)
		go bootstrapPubsub(ctx, r.cr, r.host, name)
	}
	r.mx.Unlock()

	// resolve to what we may already have in the datastore
	dsval, err := r.ds.Get(dshelp.NewKeyFromBinary([]byte(name)))
	if err != nil {
		if err == ds.ErrNotFound {
			return "", ErrResolveFailed
		}
		return "", err
	}

	data := dsval.([]byte)
	entry := new(pb.IpnsEntry)

	err = proto.Unmarshal(data, entry)
	if err != nil {
		return "", err
	}

	// check EOL; if the entry has expired, delete from datastore and return ds.ErrNotFound
	eol, ok := checkEOL(entry)
	if ok && eol.Before(time.Now()) {
		err = r.ds.Delete(dshelp.NewKeyFromBinary([]byte(name)))
		if err != nil {
			log.Warningf("PubsubResolve: error deleting stale value for %s: %s", name, err.Error())
		}

		return "", ErrResolveFailed
	}

	value, err := path.ParsePath(string(entry.GetValue()))
	return value, err
}

// GetSubscriptions retrieves a list of active topic subscriptions
func (r *PubsubResolver) GetSubscriptions() []string {
	r.mx.Lock()
	defer r.mx.Unlock()

	var res []string
	for sub := range r.subs {
		res = append(res, sub)
	}

	return res
}

// Cancel cancels a topic subscription; returns true if an active
// subscription was canceled
func (r *PubsubResolver) Cancel(name string) bool {
	r.mx.Lock()
	defer r.mx.Unlock()

	sub, ok := r.subs[name]
	if ok {
		sub.Cancel()
		delete(r.subs, name)
	}

	return ok
}

func (r *PubsubResolver) handleSubscription(sub *floodsub.Subscription, name string, pubk ci.PubKey, cancel func()) {
	defer sub.Cancel()
	defer cancel()

	for {
		msg, err := sub.Next(r.ctx)
		if err != nil {
			if err != context.Canceled {
				log.Warningf("PubsubResolve: subscription error in %s: %s", name, err.Error())
			}
			return
		}

		err = r.receive(msg, name, pubk)
		if err != nil {
			log.Warningf("PubsubResolve: error proessing update for %s: %s", name, err.Error())
		}
	}
}

func (r *PubsubResolver) receive(msg *floodsub.Message, name string, pubk ci.PubKey) error {
	data := msg.GetData()
	if data == nil {
		return errors.New("empty message")
	}

	entry := new(pb.IpnsEntry)
	err := proto.Unmarshal(data, entry)
	if err != nil {
		return err
	}

	ok, err := pubk.Verify(ipnsEntryDataForSig(entry), entry.GetSignature())
	if err != nil || !ok {
		return errors.New("signature verification failed")
	}

	_, err = path.ParsePath(string(entry.GetValue()))
	if err != nil {
		return err
	}

	eol, ok := checkEOL(entry)
	if ok && eol.Before(time.Now()) {
		return errors.New("stale update; EOL exceeded")
	}

	// check the sequence number against what we may already have in our datastore
	oval, err := r.ds.Get(dshelp.NewKeyFromBinary([]byte(name)))
	if err == nil {
		odata := oval.([]byte)
		oentry := new(pb.IpnsEntry)

		err = proto.Unmarshal(odata, oentry)
		if err != nil {
			return err
		}

		if entry.GetSequence() <= oentry.GetSequence() {
			return errors.New("stale update; sequence number too small")
		}
	}

	log.Debugf("PubsubResolve: receive IPNS record for %s", name)

	return r.ds.Put(dshelp.NewKeyFromBinary([]byte(name)), data)
}

// rendezvous with peers in the name topic through provider records
// Note: rendezbous/boostrap should really be handled by the pubsub implementation itself!
func bootstrapPubsub(ctx context.Context, cr routing.ContentRouting, host p2phost.Host, name string) {
	topic := "floodsub:" + name
	hash := u.Hash([]byte(topic))
	rz := cid.NewCidV1(cid.Raw, hash)

	err := cr.Provide(ctx, rz, true)
	if err != nil {
		log.Warningf("bootstrapPubsub: error providing rendezvous for %s: %s", topic, err.Error())
	}

	go func() {
		for {
			select {
			case <-time.After(8 * time.Hour):
				err := cr.Provide(ctx, rz, true)
				if err != nil {
					log.Warningf("bootstrapPubsub: error providing rendezvous for %s: %s", topic, err.Error())
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	rzctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	wg := &sync.WaitGroup{}
	for pi := range cr.FindProvidersAsync(rzctx, rz, 10) {
		if pi.ID == host.ID() {
			continue
		}
		wg.Add(1)
		go func(pi pstore.PeerInfo) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()

			err := host.Connect(ctx, pi)
			if err != nil {
				log.Debugf("Error connecting to pubsub peer %s: %s", pi.ID, err.Error())
				return
			}

			// delay to let pubsub perform its handshake
			time.Sleep(time.Millisecond * 250)

			log.Debugf("Connected to pubsub peer %s", pi.ID)
		}(pi)
	}

	wg.Wait()
}
