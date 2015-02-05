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

	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	host "github.com/jbenet/go-ipfs/p2p/host"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	protocol "github.com/jbenet/go-ipfs/p2p/protocol"
	routing "github.com/jbenet/go-ipfs/routing"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	kb "github.com/jbenet/go-ipfs/routing/common/kbucket"
	record "github.com/jbenet/go-ipfs/routing/common/record"
	"github.com/jbenet/go-ipfs/thirdparty/eventlog"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
)

var log = eventlog.Logger("dht")

var ProtocolDHT protocol.ID = "/ipfs/dht"

const doPinging = false

// NumBootstrapQueries defines the number of random dht queries to do to
// collect members of the routing table.
const NumBootstrapQueries = 5

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	host      host.Host      // the network services we need
	self      peer.ID        // Local peer (yourself)
	peerstore peer.Peerstore // Peer Registry

	datastore ds.ThreadSafeDatastore // Local data

	routingTable *kb.RoutingTable // Array of routing tables for differently distanced nodes
	providers    *ProviderManager

	birth    time.Time  // When this peer started up
	diaglock sync.Mutex // lock to make diagnostics work better

	Validator record.Validator // record validator funcs

	ctxgroup.ContextGroup
}

// NewDHT creates a new DHT object with the given peer as the 'local' host
func NewDHT(ctx context.Context, h host.Host, dstore ds.ThreadSafeDatastore) *IpfsDHT {
	dht := new(IpfsDHT)
	dht.datastore = dstore
	dht.self = h.ID()
	dht.peerstore = h.Peerstore()
	dht.host = h

	// register for network notifs.
	dht.host.Network().Notify((*netNotifiee)(dht))

	dht.ContextGroup = ctxgroup.WithContextAndTeardown(ctx, func() error {
		// remove ourselves from network notifs.
		dht.host.Network().StopNotify((*netNotifiee)(dht))
		return nil
	})

	h.SetStreamHandler(ProtocolDHT, dht.handleNewStream)
	dht.providers = NewProviderManager(dht.Context(), dht.self)
	dht.AddChildGroup(dht.providers)

	dht.routingTable = kb.NewRoutingTable(20, kb.ConvertPeerID(dht.self), time.Minute, dht.peerstore)
	dht.birth = time.Now()

	dht.Validator = make(record.Validator)
	dht.Validator["pk"] = record.ValidatePublicKeyRecord

	if doPinging {
		dht.Children().Add(1)
		go dht.PingRoutine(time.Second * 10)
	}
	return dht
}

// LocalPeer returns the peer.Peer of the dht.
func (dht *IpfsDHT) LocalPeer() peer.ID {
	return dht.self
}

// log returns the dht's logger
func (dht *IpfsDHT) log() eventlog.EventLogger {
	return log // TODO rm
}

// Connect to a new peer at the given address, ping and add to the routing table
func (dht *IpfsDHT) Connect(ctx context.Context, npeer peer.ID) error {
	// TODO: change interface to accept a PeerInfo as well.
	if err := dht.host.Connect(ctx, peer.PeerInfo{ID: npeer}); err != nil {
		return err
	}

	// Ping new peer to register in their routing table
	// NOTE: this should be done better...
	if _, err := dht.Ping(ctx, npeer); err != nil {
		return fmt.Errorf("failed to ping newly connected peer: %s", err)
	}
	log.Event(ctx, "connect", dht.self, npeer)
	dht.Update(ctx, npeer)
	return nil
}

// putValueToPeer stores the given key/value pair at the peer 'p'
func (dht *IpfsDHT) putValueToPeer(ctx context.Context, p peer.ID,
	key u.Key, rec *pb.Record) error {

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
func (dht *IpfsDHT) putProvider(ctx context.Context, p peer.ID, key string) error {

	// add self as the provider
	pi := peer.PeerInfo{
		ID:    dht.self,
		Addrs: dht.host.Addrs(),
	}

	// // only share WAN-friendly addresses ??
	// pi.Addrs = addrutil.WANShareableAddrs(pi.Addrs)
	if len(pi.Addrs) < 1 {
		// log.Infof("%s putProvider: %s for %s error: no wan-friendly addresses", dht.self, p, u.Key(key), pi.Addrs)
		return fmt.Errorf("no known addresses for self. cannot put provider.")
	}

	pmes := pb.NewMessage(pb.Message_ADD_PROVIDER, string(key), 0)
	pmes.ProviderPeers = pb.RawPeerInfosToPBPeers([]peer.PeerInfo{pi})
	err := dht.sendMessage(ctx, p, pmes)
	if err != nil {
		return err
	}

	log.Debugf("%s putProvider: %s for %s (%s)", dht.self, p, u.Key(key), pi.Addrs)
	return nil
}

// getValueOrPeers queries a particular peer p for the value for
// key. It returns either the value or a list of closer peers.
// NOTE: it will update the dht's peerstore with any new addresses
// it finds for the given peer.
func (dht *IpfsDHT) getValueOrPeers(ctx context.Context, p peer.ID,
	key u.Key) ([]byte, []peer.PeerInfo, error) {

	pmes, err := dht.getValueSingle(ctx, p, key)
	if err != nil {
		return nil, nil, err
	}

	if record := pmes.GetRecord(); record != nil {
		// Success! We were given the value
		log.Debug("getValueOrPeers: got value")

		// make sure record is valid.
		err = dht.verifyRecordOnline(ctx, record)
		if err != nil {
			log.Info("Received invalid record! (discarded)")
			return nil, nil, err
		}
		return record.GetValue(), nil, nil
	}

	// Perhaps we were given closer peers
	peers := pb.PBPeersToPeerInfos(pmes.GetCloserPeers())
	if len(peers) > 0 {
		log.Debug("getValueOrPeers: peers")
		return nil, peers, nil
	}

	log.Warning("getValueOrPeers: routing.ErrNotFound")
	return nil, nil, routing.ErrNotFound
}

// getValueSingle simply performs the get value RPC with the given parameters
func (dht *IpfsDHT) getValueSingle(ctx context.Context, p peer.ID,
	key u.Key) (*pb.Message, error) {
	defer log.EventBegin(ctx, "getValueSingle", p, &key).Done()

	pmes := pb.NewMessage(pb.Message_GET_VALUE, string(key), 0)
	return dht.sendRequest(ctx, p, pmes)
}

// getLocal attempts to retrieve the value from the datastore
func (dht *IpfsDHT) getLocal(key u.Key) ([]byte, error) {

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
		err = dht.verifyRecordLocally(rec)
		if err != nil {
			log.Debugf("local record verify failed: %s (discarded)", err)
			return nil, err
		}
	}

	return rec.GetValue(), nil
}

// getOwnPrivateKey attempts to load the local peers private
// key from the peerstore.
func (dht *IpfsDHT) getOwnPrivateKey() (ci.PrivKey, error) {
	sk := dht.peerstore.PrivKey(dht.self)
	if sk == nil {
		log.Warningf("%s dht cannot get own private key!", dht.self)
		return nil, fmt.Errorf("cannot get private key to sign record!")
	}
	return sk, nil
}

// putLocal stores the key value pair in the datastore
func (dht *IpfsDHT) putLocal(key u.Key, value []byte) error {
	sk, err := dht.getOwnPrivateKey()
	if err != nil {
		return err
	}

	rec, err := record.MakePutRecord(sk, key, value)
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
func (dht *IpfsDHT) Update(ctx context.Context, p peer.ID) {
	log.Event(ctx, "updatePeer", p)
	dht.routingTable.Update(p)
}

// FindLocal looks for a peer with a given ID connected to this dht and returns the peer and the table it was found in.
func (dht *IpfsDHT) FindLocal(id peer.ID) peer.PeerInfo {
	p := dht.routingTable.Find(id)
	if p != "" {
		return dht.peerstore.PeerInfo(p)
	}
	return peer.PeerInfo{}
}

// findPeerSingle asks peer 'p' if they know where the peer with id 'id' is
func (dht *IpfsDHT) findPeerSingle(ctx context.Context, p peer.ID, id peer.ID) (*pb.Message, error) {
	defer log.EventBegin(ctx, "findPeerSingle", p, id).Done()

	pmes := pb.NewMessage(pb.Message_FIND_NODE, string(id), 0)
	return dht.sendRequest(ctx, p, pmes)
}

func (dht *IpfsDHT) findProvidersSingle(ctx context.Context, p peer.ID, key u.Key) (*pb.Message, error) {
	defer log.EventBegin(ctx, "findProvidersSingle", p, &key).Done()

	pmes := pb.NewMessage(pb.Message_GET_PROVIDERS, string(key), 0)
	return dht.sendRequest(ctx, p, pmes)
}

// nearestPeersToQuery returns the routing tables closest peers.
func (dht *IpfsDHT) nearestPeersToQuery(pmes *pb.Message, count int) []peer.ID {
	key := u.Key(pmes.GetKey())
	closer := dht.routingTable.NearestPeers(kb.ConvertKey(key), count)
	return closer
}

// betterPeerToQuery returns nearestPeersToQuery, but iff closer than self.
func (dht *IpfsDHT) betterPeersToQuery(pmes *pb.Message, p peer.ID, count int) []peer.ID {
	closer := dht.nearestPeersToQuery(pmes, count)

	// no node? nil
	if closer == nil {
		return nil
	}

	// == to self? thats bad
	for _, p := range closer {
		if p == dht.self {
			log.Debug("Attempted to return self! this shouldnt happen...")
			return nil
		}
	}

	var filtered []peer.ID
	for _, clp := range closer {
		// Dont send a peer back themselves
		if p == clp {
			continue
		}

		// must all be closer than self
		key := u.Key(pmes.GetKey())
		if !kb.Closer(dht.self, clp, key) {
			filtered = append(filtered, clp)
		}
	}

	// ok seems like closer nodes
	return filtered
}

func (dht *IpfsDHT) ensureConnectedToPeer(ctx context.Context, p peer.ID) error {
	if p == dht.self {
		return errors.New("attempting to ensure connection to self")
	}

	// dial connection
	return dht.host.Connect(ctx, peer.PeerInfo{ID: p})
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
				_, err := dht.Ping(ctx, p)
				if err != nil {
					log.Debugf("Ping error: %s", err)
				}
			}
		case <-dht.Closing():
			return
		}
	}
}
