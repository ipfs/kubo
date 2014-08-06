package dht

import (
	"sync"
	"time"
	"encoding/json"

	peer	"github.com/jbenet/go-ipfs/peer"
	swarm	"github.com/jbenet/go-ipfs/swarm"
	u		"github.com/jbenet/go-ipfs/util"
	identify "github.com/jbenet/go-ipfs/identify"

	ma "github.com/jbenet/go-multiaddr"

	ds "github.com/jbenet/datastore.go"

	"code.google.com/p/goprotobuf/proto"
)

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	routes *RoutingTable

	network *swarm.Swarm

	// Local peer (yourself)
	self *peer.Peer

	// Local data
	datastore ds.Datastore

	// Map keys to peers that can provide their value
	// TODO: implement a TTL on each of these keys
	providers map[u.Key][]*peer.Peer
	providerLock sync.RWMutex

	// map of channels waiting for reply messages
	listeners  map[uint64]chan *swarm.Message
	listenLock sync.RWMutex

	// Signal to shutdown dht
	shutdown chan struct{}
}

// Create a new DHT object with the given peer as the 'local' host
func NewDHT(p *peer.Peer) (*IpfsDHT, error) {
	if p == nil {
		panic("Tried to create new dht with nil peer")
	}
	network := swarm.NewSwarm(p)
	err := network.Listen()
	if err != nil {
		return nil,err
	}

	dht := new(IpfsDHT)
	dht.network = network
	dht.datastore = ds.NewMapDatastore()
	dht.self = p
	dht.listeners = make(map[uint64]chan *swarm.Message)
	dht.providers = make(map[u.Key][]*peer.Peer)
	dht.shutdown = make(chan struct{})
	dht.routes = NewRoutingTable(20, convertPeerID(p.ID))
	return dht, nil
}

// Start up background goroutines needed by the DHT
func (dht *IpfsDHT) Start() {
	go dht.handleMessages()
}

// Connect to a new peer at the given address
// TODO: move this into swarm
func (dht *IpfsDHT) Connect(addr *ma.Multiaddr) (*peer.Peer, error) {
	if addr == nil {
		panic("addr was nil!")
	}
	peer := new(peer.Peer)
	peer.AddAddress(addr)

	conn,err := swarm.Dial("tcp", peer)
	if err != nil {
		return nil, err
	}

	err = identify.Handshake(dht.self, peer, conn.Incoming.MsgChan, conn.Outgoing.MsgChan)
	if err != nil {
		return nil, err
	}

	// Send node an address that you can be reached on
	myaddr := dht.self.NetAddress("tcp")
	mastr,err := myaddr.String()
	if err != nil {
		panic("No local address to send")
	}

	conn.Outgoing.MsgChan <- []byte(mastr)

	dht.network.StartConn(conn)

	removed := dht.routes.Update(peer)
	if removed != nil {
		panic("need to remove this peer.")
	}

	// Ping new peer to register in their routing table
	// NOTE: this should be done better...
	err = dht.Ping(peer, time.Second * 2)
	if err != nil {
		panic("Failed to ping new peer.")
	}

	return peer, nil
}

// Read in all messages from swarm and handle them appropriately
// NOTE: this function is just a quick sketch
func (dht *IpfsDHT) handleMessages() {
	u.DOut("Begin message handling routine")
	for {
		select {
		case mes,ok := <-dht.network.Chan.Incoming:
			if !ok {
				u.DOut("handleMessages closing, bad recv on incoming")
				return
			}
			pmes := new(DHTMessage)
			err := proto.Unmarshal(mes.Data, pmes)
			if err != nil {
				u.PErr("Failed to decode protobuf message: %s", err)
				continue
			}

			// Update peers latest visit in routing table
			removed := dht.routes.Update(mes.Peer)
			if removed != nil {
				panic("Need to handle removed peer.")
			}

			// Note: not sure if this is the correct place for this
			if pmes.GetResponse() {
				dht.listenLock.RLock()
				ch, ok := dht.listeners[pmes.GetId()]
				dht.listenLock.RUnlock()
				if ok {
					ch <- mes
				} else {
					// this is expected behaviour during a timeout
					u.DOut("Received response with nobody listening...")
				}

				continue
			}
			//

			u.DOut("Got message type: '%s' [id = %x]", DHTMessage_MessageType_name[int32(pmes.GetType())], pmes.GetId())
			switch pmes.GetType() {
			case DHTMessage_GET_VALUE:
				dht.handleGetValue(mes.Peer, pmes)
			case DHTMessage_PUT_VALUE:
				dht.handlePutValue(mes.Peer, pmes)
			case DHTMessage_FIND_NODE:
				dht.handleFindPeer(mes.Peer, pmes)
			case DHTMessage_ADD_PROVIDER:
				dht.handleAddProvider(mes.Peer, pmes)
			case DHTMessage_GET_PROVIDERS:
				dht.handleGetProviders(mes.Peer, pmes)
			case DHTMessage_PING:
				dht.handlePing(mes.Peer, pmes)
			}

		case err := <-dht.network.Chan.Errors:
			u.DErr("dht err: %s", err)
		case <-dht.shutdown:
			return
		}
	}
}

func (dht *IpfsDHT) handleGetValue(p *peer.Peer, pmes *DHTMessage) {
	dskey := ds.NewKey(pmes.GetKey())
	i_val, err := dht.datastore.Get(dskey)
	if err == nil {
		resp := &pDHTMessage{
			Response: true,
			Id: *pmes.Id,
			Key: *pmes.Key,
			Value: i_val.([]byte),
		}

		mes := swarm.NewMessage(p, resp.ToProtobuf())
		dht.network.Chan.Outgoing <- mes
	} else if err == ds.ErrNotFound {
		// Find closest peer(s) to desired key and reply with that info
		// TODO: this will need some other metadata in the protobuf message
		//			to signal to the querying peer that the data its receiving
		//			is actually a list of other peer
	}
}

// Store a value in this peer local storage
func (dht *IpfsDHT) handlePutValue(p *peer.Peer, pmes *DHTMessage) {
	dskey := ds.NewKey(pmes.GetKey())
	err := dht.datastore.Put(dskey, pmes.GetValue())
	if err != nil {
		// For now, just panic, handle this better later maybe
		panic(err)
	}
}

func (dht *IpfsDHT) handlePing(p *peer.Peer, pmes *DHTMessage) {
	resp := pDHTMessage{
		Type: pmes.GetType(),
		Response: true,
		Id: pmes.GetId(),
	}

	dht.network.Chan.Outgoing <-swarm.NewMessage(p, resp.ToProtobuf())
}

func (dht *IpfsDHT) handleFindPeer(p *peer.Peer, pmes *DHTMessage) {
	u.POut("handleFindPeer: searching for '%s'", peer.ID(pmes.GetKey()).Pretty())
	closest := dht.routes.NearestPeer(convertKey(u.Key(pmes.GetKey())))
	if closest == nil {
		panic("could not find anything.")
	}

	if len(closest.Addresses) == 0 {
		panic("no addresses for connected peer...")
	}

	u.POut("handleFindPeer: sending back '%s'", closest.ID.Pretty())

	addr,err := closest.Addresses[0].String()
	if err != nil {
		panic(err)
	}

	resp := pDHTMessage{
		Type: pmes.GetType(),
		Response: true,
		Id: pmes.GetId(),
		Value: []byte(addr),
	}

	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.network.Chan.Outgoing <-mes
}

func (dht *IpfsDHT) handleGetProviders(p *peer.Peer, pmes *DHTMessage) {
	dht.providerLock.RLock()
	providers := dht.providers[u.Key(pmes.GetKey())]
	dht.providerLock.RUnlock()
	if providers == nil || len(providers) == 0 {
		// ?????
		u.DOut("No known providers for requested key.")
	}

	// This is just a quick hack, formalize method of sending addrs later
	addrs := make(map[u.Key]string)
	for _,prov := range providers {
		ma := prov.NetAddress("tcp")
		str,err := ma.String()
		if err != nil {
			u.PErr("Error: %s", err)
			continue
		}

		addrs[prov.Key()] = str
	}

	data,err := json.Marshal(addrs)
	if err != nil {
		panic(err)
	}

	resp := pDHTMessage{
		Type: DHTMessage_GET_PROVIDERS,
		Key: pmes.GetKey(),
		Value: data,
		Id: pmes.GetId(),
		Response: true,
	}

	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.network.Chan.Outgoing <-mes
}

func (dht *IpfsDHT) handleAddProvider(p *peer.Peer, pmes *DHTMessage) {
	//TODO: need to implement TTLs on providers
	key := u.Key(pmes.GetKey())
	dht.addProviderEntry(key, p)
}


// Register a handler for a specific message ID, used for getting replies
// to certain messages (i.e. response to a GET_VALUE message)
func (dht *IpfsDHT) ListenFor(mesid uint64) <-chan *swarm.Message {
	lchan := make(chan *swarm.Message)
	dht.listenLock.Lock()
	dht.listeners[mesid] = lchan
	dht.listenLock.Unlock()
	return lchan
}

// Unregister the given message id from the listener map
func (dht *IpfsDHT) Unlisten(mesid uint64) {
	dht.listenLock.Lock()
	ch, ok := dht.listeners[mesid]
	if ok {
		delete(dht.listeners, mesid)
	}
	dht.listenLock.Unlock()
	close(ch)
}

// Stop all communications from this peer and shut down
func (dht *IpfsDHT) Halt() {
	dht.shutdown <- struct{}{}
	dht.network.Close()
}

// Ping a peer, log the time it took
func (dht *IpfsDHT) Ping(p *peer.Peer, timeout time.Duration) error {
	// Thoughts: maybe this should accept an ID and do a peer lookup?
	u.DOut("Enter Ping.")

	pmes := pDHTMessage{Id: GenerateMessageID(), Type: DHTMessage_PING}
	mes := swarm.NewMessage(p, pmes.ToProtobuf())

	before := time.Now()
	response_chan := dht.ListenFor(pmes.Id)
	dht.network.Chan.Outgoing <- mes

	tout := time.After(timeout)
	select {
	case <-response_chan:
		roundtrip := time.Since(before)
		u.POut("Ping took %s.", roundtrip.String())
		return nil
	case <-tout:
		// Timed out, think about removing peer from network
		u.DOut("Ping peer timed out.")
		return u.ErrTimeout
	}
}

func (dht *IpfsDHT) addProviderEntry(key u.Key, p *peer.Peer) {
	u.DOut("Adding %s as provider for '%s'", p.Key().Pretty(), key)
	dht.providerLock.Lock()
	provs := dht.providers[key]
	dht.providers[key] = append(provs, p)
	dht.providerLock.Unlock()
}
