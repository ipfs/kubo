// Package dht implements a distributed hash table that satisfies the ipfs routing
// interface. This DHT is modeled after kademlia with Coral and S/Kademlia modifications.
package dht

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	host "github.com/ipfs/go-ipfs/p2p/host"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	protocol "github.com/ipfs/go-ipfs/p2p/protocol"
	routing "github.com/ipfs/go-ipfs/routing"
	pb "github.com/ipfs/go-ipfs/routing/dht/pb"
	kb "github.com/ipfs/go-ipfs/routing/kbucket"
	record "github.com/ipfs/go-ipfs/routing/record"
	"github.com/ipfs/go-ipfs/thirdparty/eventlog"
	u "github.com/ipfs/go-ipfs/util"

	proto "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	goprocess "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	goprocessctx "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/context"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

var log = eventlog.Logger("dht")

var ProtocolDHT protocol.ID = "/ipfs/dht"

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

	datastore ds.Datastore // Local data

	routingTable *kb.RoutingTable // Array of routing tables for differently distanced nodes
	providers    *ProviderManager

	birth    time.Time  // When this peer started up
	diaglock sync.Mutex // lock to make diagnostics work better

	Validator record.Validator // record validator funcs

	ctx  context.Context
	proc goprocess.Process
}

// NewDHT creates a new DHT object with the given peer as the 'local' host
func NewDHT(ctx context.Context, h host.Host, dstore ds.Datastore) *IpfsDHT {
	dht := new(IpfsDHT)
	dht.datastore = dstore
	dht.self = h.ID()
	dht.peerstore = h.Peerstore()
	dht.host = h

	// register for network notifs.
	dht.host.Network().Notify((*netNotifiee)(dht))

	dht.proc = goprocess.WithTeardown(func() error {
		// remove ourselves from network notifs.
		dht.host.Network().StopNotify((*netNotifiee)(dht))
		return nil
	})

	dht.ctx = ctx

	h.SetStreamHandler(ProtocolDHT, dht.handleNewStream)
	dht.providers = NewProviderManager(dht.ctx, dht.self)
	dht.proc.AddChild(dht.providers.proc)
	goprocessctx.CloseAfterContext(dht.proc, ctx)

	dht.routingTable = kb.NewRoutingTable(20, kb.ConvertPeerID(dht.self), time.Minute, dht.peerstore)
	dht.birth = time.Now()

	dht.Validator = make(record.Validator)
	dht.Validator["pk"] = record.PublicKeyValidator

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

// putValueToPeer stores the given key/value pair at the peer 'p'
func (dht *IpfsDHT) putValueToPeer(ctx context.Context, p peer.ID,
	key key.Key, rec *pb.Record) error {

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
func (dht *IpfsDHT) putProvider(ctx context.Context, p peer.ID, skey string) error {

	// add self as the provider
	pi := peer.PeerInfo{
		ID:    dht.self,
		Addrs: dht.host.Addrs(),
	}

	// // only share WAN-friendly addresses ??
	// pi.Addrs = addrutil.WANShareableAddrs(pi.Addrs)
	if len(pi.Addrs) < 1 {
		// log.Infof("%s putProvider: %s for %s error: no wan-friendly addresses", dht.self, p, key.Key(key), pi.Addrs)
		return fmt.Errorf("no known addresses for self. cannot put provider.")
	}

	pmes := pb.NewMessage(pb.Message_ADD_PROVIDER, skey, 0)
	pmes.ProviderPeers = pb.RawPeerInfosToPBPeers([]peer.PeerInfo{pi})
	err := dht.sendMessage(ctx, p, pmes)
	if err != nil {
		return err
	}

	log.Debugf("%s putProvider: %s for %s (%s)", dht.self, p, key.Key(skey), pi.Addrs)
	return nil
}

// getValueOrPeers queries a particular peer p for the value for
// key. It returns either the value or a list of closer peers.
// NOTE: it will update the dht's peerstore with any new addresses
// it finds for the given peer.
func (dht *IpfsDHT) getValueOrPeers(ctx context.Context, p peer.ID,
	key key.Key) ([]byte, []peer.PeerInfo, error) {

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
	key key.Key) (*pb.Message, error) {
	defer log.EventBegin(ctx, "getValueSingle", p, &key).Done()

	pmes := pb.NewMessage(pb.Message_GET_VALUE, string(key), 0)
	return dht.sendRequest(ctx, p, pmes)
}

// getLocal attempts to retrieve the value from the datastore
func (dht *IpfsDHT) getLocal(key key.Key) ([]byte, error) {

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
func (dht *IpfsDHT) putLocal(key key.Key, rec *pb.Record) error {
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

func (dht *IpfsDHT) findProvidersSingle(ctx context.Context, p peer.ID, key key.Key) (*pb.Message, error) {
	defer log.EventBegin(ctx, "findProvidersSingle", p, &key).Done()

	pmes := pb.NewMessage(pb.Message_GET_PROVIDERS, string(key), 0)
	return dht.sendRequest(ctx, p, pmes)
}

// nearestPeersToQuery returns the routing tables closest peers.
func (dht *IpfsDHT) nearestPeersToQuery(pmes *pb.Message, count int) []peer.ID {
	key := key.Key(pmes.GetKey())
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
		key := key.Key(pmes.GetKey())
		if !kb.Closer(dht.self, clp, key) {
			filtered = append(filtered, clp)
		}
	}

	// ok seems like closer nodes
	return filtered
}

// Context return dht's context
func (dht *IpfsDHT) Context() context.Context {
	return dht.ctx
}

// Process return dht's process
func (dht *IpfsDHT) Process() goprocess.Process {
	return dht.proc
}

// Close calls Process Close
func (dht *IpfsDHT) Close() error {
	return dht.proc.Close()
}
