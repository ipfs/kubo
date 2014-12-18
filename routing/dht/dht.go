// Package dht implements a distributed hash table that satisfies the ipfs routing
// interface. This DHT is modeled after kademlia with Coral and S/Kademlia modifications.
package dht

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/eventlog"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
)

var log = eventlog.Logger("dht")

const doPinging = false

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	network   inet.Network   // the network services we need
	self      peer.Peer      // Local peer (yourself)
	peerstore peer.Peerstore // Other peers

	datastore ds.Datastore // Local data
	dslock    sync.Mutex

	routingTable *kb.RoutingTable // Array of routing tables for differently distanced nodes
	providers    *ProviderManager

	birth    time.Time  // When this peer started up
	diaglock sync.Mutex // lock to make diagnostics work better

	// record validator funcs
	Validators map[string]ValidatorFunc

	ctxgroup.ContextGroup
}

// NewDHT creates a new DHT object with the given peer as the 'local' host
func NewDHT(ctx context.Context, p peer.Peer, ps peer.Peerstore, n inet.Network, dstore ds.Datastore) *IpfsDHT {
	dht := new(IpfsDHT)
	dht.datastore = dstore
	dht.self = p
	dht.peerstore = ps
	dht.ContextGroup = ctxgroup.WithContext(ctx)
	dht.network = n
	n.SetHandler(inet.ProtocolDHT, dht.handleNewStream)

	dht.providers = NewProviderManager(dht.Context(), p.ID())
	dht.AddChildGroup(dht.providers)

	dht.routingTable = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID()), time.Minute)
	dht.birth = time.Now()

	dht.Validators = make(map[string]ValidatorFunc)
	dht.Validators["pk"] = ValidatePublicKeyRecord

	if doPinging {
		dht.Children().Add(1)
		go dht.PingRoutine(time.Second * 10)
	}
	return dht
}

// Connect to a new peer at the given address, ping and add to the routing table
func (dht *IpfsDHT) Connect(ctx context.Context, npeer peer.Peer) error {
	if err := dht.network.DialPeer(ctx, npeer); err != nil {
		return err
	}

	// Ping new peer to register in their routing table
	// NOTE: this should be done better...
	if err := dht.Ping(ctx, npeer); err != nil {
		return fmt.Errorf("failed to ping newly connected peer: %s\n", err)
	}
	log.Event(ctx, "connect", dht.self, npeer)
	dht.Update(ctx, npeer)
	return nil
}

// putValueToNetwork stores the given key/value pair at the peer 'p'
func (dht *IpfsDHT) putValueToNetwork(ctx context.Context, p peer.Peer,
	key string, rec *pb.Record) error {

	pmes := pb.NewMessage(pb.Message_PUT_VALUE, string(key), 0)
	pmes.Record = rec
	rpmes, err := dht.sendRequest(ctx, p, pmes)
	if err != nil {
		return err
	}

	if !bytes.Equal(rpmes.GetRecord().Value, pmes.GetRecord().Value) {
		return errors.New("value not put correctly")
	}
	return nil
}

// putProvider sends a message to peer 'p' saying that the local node
// can provide the value of 'key'
func (dht *IpfsDHT) putProvider(ctx context.Context, p peer.Peer, key string) error {

	pmes := pb.NewMessage(pb.Message_ADD_PROVIDER, string(key), 0)

	// add self as the provider
	pmes.ProviderPeers = pb.PeersToPBPeers(dht.network, []peer.Peer{dht.self})

	err := dht.sendMessage(ctx, p, pmes)
	if err != nil {
		return err
	}

	log.Debugf("%s putProvider: %s for %s", dht.self, p, u.Key(key))

	return nil
}

func (dht *IpfsDHT) getValueOrPeers(ctx context.Context, p peer.Peer,
	key u.Key) ([]byte, []peer.Peer, error) {

	pmes, err := dht.getValueSingle(ctx, p, key)
	if err != nil {
		return nil, nil, err
	}

	if record := pmes.GetRecord(); record != nil {
		// Success! We were given the value
		log.Debug("getValueOrPeers: got value")

		// make sure record is still valid
		err = dht.verifyRecord(record)
		if err != nil {
			log.Error("Received invalid record!")
			return nil, nil, err
		}
		return record.GetValue(), nil, nil
	}

	// TODO decide on providers. This probably shouldn't be happening.
	if prv := pmes.GetProviderPeers(); prv != nil && len(prv) > 0 {
		val, err := dht.getFromPeerList(ctx, key, prv)
		if err != nil {
			return nil, nil, err
		}
		log.Debug("getValueOrPeers: get from providers")
		return val, nil, nil
	}

	// Perhaps we were given closer peers
	peers, errs := pb.PBPeersToPeers(dht.peerstore, pmes.GetCloserPeers())
	for _, err := range errs {
		if err != nil {
			log.Error(err)
		}
	}

	if len(peers) > 0 {
		log.Debug("getValueOrPeers: peers")
		return nil, peers, nil
	}

	log.Warning("getValueOrPeers: routing.ErrNotFound")
	return nil, nil, routing.ErrNotFound
}

// getValueSingle simply performs the get value RPC with the given parameters
func (dht *IpfsDHT) getValueSingle(ctx context.Context, p peer.Peer,
	key u.Key) (*pb.Message, error) {

	pmes := pb.NewMessage(pb.Message_GET_VALUE, string(key), 0)
	return dht.sendRequest(ctx, p, pmes)
}

// TODO: Im not certain on this implementation, we get a list of peers/providers
// from someone what do we do with it? Connect to each of them? randomly pick
// one to get the value from? Or just connect to one at a time until we get a
// successful connection and request the value from it?
func (dht *IpfsDHT) getFromPeerList(ctx context.Context, key u.Key,
	peerlist []*pb.Message_Peer) ([]byte, error) {

	for _, pinfo := range peerlist {
		p, err := dht.ensureConnectedToPeer(ctx, pinfo)
		if err != nil {
			log.Errorf("getFromPeers error: %s", err)
			continue
		}

		pmes, err := dht.getValueSingle(ctx, p, key)
		if err != nil {
			log.Errorf("getFromPeers error: %s\n", err)
			continue
		}

		if record := pmes.GetRecord(); record != nil {
			// Success! We were given the value

			err := dht.verifyRecord(record)
			if err != nil {
				return nil, err
			}
			dht.providers.AddProvider(key, p)
			return record.GetValue(), nil
		}
	}
	return nil, routing.ErrNotFound
}

// getLocal attempts to retrieve the value from the datastore
func (dht *IpfsDHT) getLocal(key u.Key) ([]byte, error) {
	dht.dslock.Lock()
	defer dht.dslock.Unlock()
	log.Debug("getLocal %s", key)
	v, err := dht.datastore.Get(key.DsKey())
	if err != nil {
		return nil, err
	}
	log.Debug("found in db")

	byt, ok := v.([]byte)
	if !ok {
		return nil, errors.New("value stored in datastore not []byte")
	}
	rec := new(pb.Record)
	err = proto.Unmarshal(byt, rec)
	if err != nil {
		return nil, err
	}

	// TODO: 'if paranoid'
	if u.Debug {
		err = dht.verifyRecord(rec)
		if err != nil {
			log.Errorf("local record verify failed: %s", err)
			return nil, err
		}
	}

	return rec.GetValue(), nil
}

// putLocal stores the key value pair in the datastore
func (dht *IpfsDHT) putLocal(key u.Key, value []byte) error {
	rec, err := dht.makePutRecord(key, value)
	if err != nil {
		return err
	}
	data, err := proto.Marshal(rec)
	if err != nil {
		return err
	}

	return dht.datastore.Put(key.DsKey(), data)
}

// Update signals the routingTable to Update its last-seen status
// on the given peer.
func (dht *IpfsDHT) Update(ctx context.Context, p peer.Peer) {
	log.Event(ctx, "updatePeer", p)
	dht.routingTable.Update(p)
}

// FindLocal looks for a peer with a given ID connected to this dht and returns the peer and the table it was found in.
func (dht *IpfsDHT) FindLocal(id peer.ID) (peer.Peer, *kb.RoutingTable) {
	p := dht.routingTable.Find(id)
	if p != nil {
		return p, dht.routingTable
	}
	return nil, nil
}

// findPeerSingle asks peer 'p' if they know where the peer with id 'id' is
func (dht *IpfsDHT) findPeerSingle(ctx context.Context, p peer.Peer, id peer.ID) (*pb.Message, error) {
	pmes := pb.NewMessage(pb.Message_FIND_NODE, string(id), 0)
	return dht.sendRequest(ctx, p, pmes)
}

func (dht *IpfsDHT) findProvidersSingle(ctx context.Context, p peer.Peer, key u.Key) (*pb.Message, error) {
	pmes := pb.NewMessage(pb.Message_GET_PROVIDERS, string(key), 0)
	return dht.sendRequest(ctx, p, pmes)
}

func (dht *IpfsDHT) addProviders(key u.Key, pbps []*pb.Message_Peer) []peer.Peer {
	peers, errs := pb.PBPeersToPeers(dht.peerstore, pbps)
	for _, err := range errs {
		log.Errorf("error converting peer: %v", err)
	}

	var provArr []peer.Peer
	for _, p := range peers {
		// Dont add outselves to the list
		if p.ID().Equal(dht.self.ID()) {
			continue
		}

		log.Debugf("%s adding provider: %s for %s", dht.self, p, key)
		// TODO(jbenet) ensure providers is idempotent
		dht.providers.AddProvider(key, p)
		provArr = append(provArr, p)
	}
	return provArr
}

// nearestPeersToQuery returns the routing tables closest peers.
func (dht *IpfsDHT) nearestPeersToQuery(pmes *pb.Message, count int) []peer.Peer {
	key := u.Key(pmes.GetKey())
	closer := dht.routingTable.NearestPeers(kb.ConvertKey(key), count)
	return closer
}

// betterPeerToQuery returns nearestPeersToQuery, but iff closer than self.
func (dht *IpfsDHT) betterPeersToQuery(pmes *pb.Message, count int) []peer.Peer {
	closer := dht.nearestPeersToQuery(pmes, count)

	// no node? nil
	if closer == nil {
		return nil
	}

	// == to self? thats bad
	for _, p := range closer {
		if p.ID().Equal(dht.self.ID()) {
			log.Error("Attempted to return self! this shouldnt happen...")
			return nil
		}
	}

	var filtered []peer.Peer
	for _, p := range closer {
		// must all be closer than self
		key := u.Key(pmes.GetKey())
		if !kb.Closer(dht.self.ID(), p.ID(), key) {
			filtered = append(filtered, p)
		}
	}

	// ok seems like closer nodes
	return filtered
}

// getPeer searches the peerstore for a peer with the given peer ID
func (dht *IpfsDHT) getPeer(id peer.ID) (peer.Peer, error) {
	p, err := dht.peerstore.FindOrCreate(id)
	if err != nil {
		err = fmt.Errorf("Failed to get peer from peerstore: %s", err)
		log.Error(err)
		return nil, err
	}
	return p, nil
}

func (dht *IpfsDHT) ensureConnectedToPeer(ctx context.Context, pbp *pb.Message_Peer) (peer.Peer, error) {
	p, err := pb.PBPeerToPeer(dht.peerstore, pbp)
	if err != nil {
		return nil, err
	}

	if dht.self.ID().Equal(p.ID()) {
		return nil, errors.New("attempting to ensure connection to self")
	}

	// dial connection
	err = dht.network.DialPeer(ctx, p)
	return p, err
}

//TODO: this should be smarter about which keys it selects.
func (dht *IpfsDHT) loadProvidableKeys() error {
	kl, err := dht.datastore.KeyList()
	if err != nil {
		return err
	}
	for _, dsk := range kl {
		k := u.KeyFromDsKey(dsk)
		if len(k) == 0 {
			log.Errorf("loadProvidableKeys error: %v", dsk)
		}

		dht.providers.AddProvider(k, dht.self)
	}
	return nil
}

// PingRoutine periodically pings nearest neighbors.
func (dht *IpfsDHT) PingRoutine(t time.Duration) {
	defer dht.Children().Done()

	tick := time.Tick(t)
	for {
		select {
		case <-tick:
			id := make([]byte, 16)
			rand.Read(id)
			peers := dht.routingTable.NearestPeers(kb.ConvertKey(u.Key(id)), 5)
			for _, p := range peers {
				ctx, _ := context.WithTimeout(dht.Context(), time.Second*5)
				err := dht.Ping(ctx, p)
				if err != nil {
					log.Errorf("Ping error: %s", err)
				}
			}
		case <-dht.Closing():
			return
		}
	}
}

// Bootstrap builds up list of peers by requesting random peer IDs
func (dht *IpfsDHT) Bootstrap(ctx context.Context) {
	id := make([]byte, 16)
	rand.Read(id)
	p, err := dht.FindPeer(ctx, peer.ID(id))
	if err != nil {
		log.Errorf("Bootstrap peer error: %s", err)
	}
	err = dht.network.DialPeer(ctx, p)
	if err != nil {
		log.Errorf("Bootstrap peer error: %s", err)
	}
}
