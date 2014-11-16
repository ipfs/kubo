// package dht implements a distributed hash table that satisfies the ipfs routing
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
	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"
	"github.com/jbenet/go-ipfs/util/eventlog"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
)

var log = eventlog.Logger("dht")

const doPinging = false

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	// Array of routing tables for differently distanced nodes
	// NOTE: (currently, only a single table is used)
	routingTables []*kb.RoutingTable

	// the network services we need
	dialer inet.Dialer
	sender inet.Sender

	// Local peer (yourself)
	self peer.Peer

	// Other peers
	peerstore peer.Peerstore

	// Local data
	datastore ds.Datastore
	dslock    sync.Mutex

	providers *ProviderManager

	// When this peer started up
	birth time.Time

	//lock to make diagnostics work better
	diaglock sync.Mutex

	// record validator funcs
	Validators map[string]ValidatorFunc

	ctxc.ContextCloser
}

// NewDHT creates a new DHT object with the given peer as the 'local' host
func NewDHT(ctx context.Context, p peer.Peer, ps peer.Peerstore, dialer inet.Dialer, sender inet.Sender, dstore ds.Datastore) *IpfsDHT {
	dht := new(IpfsDHT)
	dht.dialer = dialer
	dht.sender = sender
	dht.datastore = dstore
	dht.self = p
	dht.peerstore = ps
	dht.ContextCloser = ctxc.NewContextCloser(ctx, nil)

	dht.providers = NewProviderManager(dht.Context(), p.ID())
	dht.AddCloserChild(dht.providers)

	dht.routingTables = make([]*kb.RoutingTable, 3)
	dht.routingTables[0] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID()), time.Millisecond*1000)
	dht.routingTables[1] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID()), time.Millisecond*1000)
	dht.routingTables[2] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID()), time.Hour)
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
func (dht *IpfsDHT) Connect(ctx context.Context, npeer peer.Peer) (peer.Peer, error) {
	log.Debugf("Connect to new peer: %s", npeer)

	// TODO(jbenet,whyrusleeping)
	//
	// Connect should take in a Peer (with ID). In a sense, we shouldn't be
	// allowing connections to random multiaddrs without knowing who we're
	// speaking to (i.e. peer.ID). In terms of moving around simple addresses
	// -- instead of an (ID, Addr) pair -- we can use:
	//
	//   /ip4/10.20.30.40/tcp/1234/ipfs/Qxhxxchxzcncxnzcnxzcxzm
	//
	err := dht.dialer.DialPeer(ctx, npeer)
	if err != nil {
		return nil, err
	}

	// Ping new peer to register in their routing table
	// NOTE: this should be done better...
	err = dht.Ping(ctx, npeer)
	if err != nil {
		return nil, fmt.Errorf("failed to ping newly connected peer: %s\n", err)
	}

	dht.Update(npeer)

	return npeer, nil
}

// HandleMessage implements the inet.Handler interface.
func (dht *IpfsDHT) HandleMessage(ctx context.Context, mes msg.NetMessage) msg.NetMessage {

	mData := mes.Data()
	if mData == nil {
		log.Error("Message contained nil data.")
		return nil
	}

	mPeer := mes.Peer()
	if mPeer == nil {
		log.Error("Message contained nil peer.")
		return nil
	}

	// deserialize msg
	pmes := new(pb.Message)
	err := proto.Unmarshal(mData, pmes)
	if err != nil {
		log.Error("Error unmarshaling data")
		return nil
	}

	// update the peer (on valid msgs only)
	dht.Update(mPeer)

	log.Event(ctx, "foo", dht.self, mPeer, pmes)

	// get handler for this msg type.
	handler := dht.handlerForMsgType(pmes.GetType())
	if handler == nil {
		log.Error("got back nil handler from handlerForMsgType")
		return nil
	}

	// dispatch handler.
	rpmes, err := handler(mPeer, pmes)
	if err != nil {
		log.Errorf("handle message error: %s", err)
		return nil
	}

	// if nil response, return it before serializing
	if rpmes == nil {
		log.Warning("Got back nil response from request.")
		return nil
	}

	// serialize response msg
	rmes, err := msg.FromObject(mPeer, rpmes)
	if err != nil {
		log.Errorf("serialze response error: %s", err)
		return nil
	}

	return rmes
}

// sendRequest sends out a request using dht.sender, but also makes sure to
// measure the RTT for latency measurements.
func (dht *IpfsDHT) sendRequest(ctx context.Context, p peer.Peer, pmes *pb.Message) (*pb.Message, error) {

	mes, err := msg.FromObject(p, pmes)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	log.Event(ctx, "sentMessage", dht.self, p, pmes)

	rmes, err := dht.sender.SendRequest(ctx, mes)
	if err != nil {
		return nil, err
	}
	if rmes == nil {
		return nil, errors.New("no response to request")
	}

	rtt := time.Since(start)
	rmes.Peer().SetLatency(rtt)

	rpmes := new(pb.Message)
	if err := proto.Unmarshal(rmes.Data(), rpmes); err != nil {
		return nil, err
	}

	return rpmes, nil
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
	pmes.ProviderPeers = pb.PeersToPBPeers([]peer.Peer{dht.self})

	rpmes, err := dht.sendRequest(ctx, p, pmes)
	if err != nil {
		return err
	}

	log.Debugf("%s putProvider: %s for %s", dht.self, p, u.Key(key))
	if rpmes.GetKey() != pmes.GetKey() {
		return errors.New("provider not added correctly")
	}

	return nil
}

func (dht *IpfsDHT) getValueOrPeers(ctx context.Context, p peer.Peer,
	key u.Key, level int) ([]byte, []peer.Peer, error) {

	pmes, err := dht.getValueSingle(ctx, p, key, level)
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
		val, err := dht.getFromPeerList(ctx, key, prv, level)
		if err != nil {
			return nil, nil, err
		}
		log.Debug("getValueOrPeers: get from providers")
		return val, nil, nil
	}

	// Perhaps we were given closer peers
	var peers []peer.Peer
	for _, pb := range pmes.GetCloserPeers() {
		pr, err := dht.peerFromInfo(pb)
		if err != nil {
			log.Error(err)
			continue
		}
		peers = append(peers, pr)
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
	key u.Key, level int) (*pb.Message, error) {

	pmes := pb.NewMessage(pb.Message_GET_VALUE, string(key), level)
	return dht.sendRequest(ctx, p, pmes)
}

// TODO: Im not certain on this implementation, we get a list of peers/providers
// from someone what do we do with it? Connect to each of them? randomly pick
// one to get the value from? Or just connect to one at a time until we get a
// successful connection and request the value from it?
func (dht *IpfsDHT) getFromPeerList(ctx context.Context, key u.Key,
	peerlist []*pb.Message_Peer, level int) ([]byte, error) {

	for _, pinfo := range peerlist {
		p, err := dht.ensureConnectedToPeer(ctx, pinfo)
		if err != nil {
			log.Errorf("getFromPeers error: %s", err)
			continue
		}

		pmes, err := dht.getValueSingle(ctx, p, key, level)
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

// Update signals to all routingTables to Update their last-seen status
// on the given peer.
func (dht *IpfsDHT) Update(p peer.Peer) {
	log.Debugf("updating peer: %s latency = %f\n", p, p.GetLatency().Seconds())
	removedCount := 0
	for _, route := range dht.routingTables {
		removed := route.Update(p)
		// Only close the connection if no tables refer to this peer
		if removed != nil {
			removedCount++
		}
	}

	// Only close the connection if no tables refer to this peer
	// if removedCount == len(dht.routingTables) {
	// 	dht.network.ClosePeer(p)
	// }
	// ACTUALLY, no, let's not just close the connection. it may be connected
	// due to other things. it seems that we just need connection timeouts
	// after some deadline of inactivity.
}

// FindLocal looks for a peer with a given ID connected to this dht and returns the peer and the table it was found in.
func (dht *IpfsDHT) FindLocal(id peer.ID) (peer.Peer, *kb.RoutingTable) {
	for _, table := range dht.routingTables {
		p := table.Find(id)
		if p != nil {
			return p, table
		}
	}
	return nil, nil
}

// findPeerSingle asks peer 'p' if they know where the peer with id 'id' is
func (dht *IpfsDHT) findPeerSingle(ctx context.Context, p peer.Peer, id peer.ID, level int) (*pb.Message, error) {
	pmes := pb.NewMessage(pb.Message_FIND_NODE, string(id), level)
	return dht.sendRequest(ctx, p, pmes)
}

func (dht *IpfsDHT) findProvidersSingle(ctx context.Context, p peer.Peer, key u.Key, level int) (*pb.Message, error) {
	pmes := pb.NewMessage(pb.Message_GET_PROVIDERS, string(key), level)
	return dht.sendRequest(ctx, p, pmes)
}

func (dht *IpfsDHT) addProviders(key u.Key, peers []*pb.Message_Peer) []peer.Peer {
	var provArr []peer.Peer
	for _, prov := range peers {
		p, err := dht.peerFromInfo(prov)
		if err != nil {
			log.Errorf("error getting peer from info: %v", err)
			continue
		}

		log.Debugf("%s adding provider: %s for %s", dht.self, p, key)

		// Dont add outselves to the list
		if p.ID().Equal(dht.self.ID()) {
			continue
		}

		// TODO(jbenet) ensure providers is idempotent
		dht.providers.AddProvider(key, p)
		provArr = append(provArr, p)
	}
	return provArr
}

// nearestPeersToQuery returns the routing tables closest peers.
func (dht *IpfsDHT) nearestPeersToQuery(pmes *pb.Message, count int) []peer.Peer {
	level := pmes.GetClusterLevel()
	cluster := dht.routingTables[level]

	key := u.Key(pmes.GetKey())
	closer := cluster.NearestPeers(kb.ConvertKey(key), count)
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
	p, err := dht.peerstore.Get(id)
	if err != nil {
		err = fmt.Errorf("Failed to get peer from peerstore: %s", err)
		log.Error(err)
		return nil, err
	}
	return p, nil
}

// peerFromInfo returns a peer using info in the protobuf peer struct
// to lookup or create a peer
func (dht *IpfsDHT) peerFromInfo(pbp *pb.Message_Peer) (peer.Peer, error) {

	id := peer.ID(pbp.GetId())

	// bail out if it's ourselves
	//TODO(jbenet) not sure this should be an error _here_
	if id.Equal(dht.self.ID()) {
		return nil, errors.New("found self")
	}

	p, err := dht.getPeer(id)
	if err != nil {
		return nil, err
	}

	maddr, err := pbp.Address()
	if err != nil {
		return nil, err
	}
	p.AddAddress(maddr)
	return p, nil
}

func (dht *IpfsDHT) ensureConnectedToPeer(ctx context.Context, pbp *pb.Message_Peer) (peer.Peer, error) {
	p, err := dht.peerFromInfo(pbp)
	if err != nil {
		return nil, err
	}

	// dial connection
	err = dht.dialer.DialPeer(ctx, p)
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
			peers := dht.routingTables[0].NearestPeers(kb.ConvertKey(u.Key(id)), 5)
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
		log.Error("Bootstrap peer error: %s", err)
	}
	err = dht.dialer.DialPeer(ctx, p)
	if err != nil {
		log.Errorf("Bootstrap peer error: %s", err)
	}
}
