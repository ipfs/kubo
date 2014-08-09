package dht

import (
	"bytes"
	"encoding/json"
	"errors"
	"sync"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"

	ma "github.com/jbenet/go-multiaddr"

	ds "github.com/jbenet/datastore.go"

	"code.google.com/p/goprotobuf/proto"
)

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	// Array of routing tables for differently distanced nodes
	// NOTE: (currently, only a single table is used)
	routes []*kb.RoutingTable

	network *swarm.Swarm

	// Local peer (yourself)
	self *peer.Peer

	// Local data
	datastore ds.Datastore

	// Map keys to peers that can provide their value
	// TODO: implement a TTL on each of these keys
	providers    map[u.Key][]*providerInfo
	providerLock sync.RWMutex

	// map of channels waiting for reply messages
	listeners  map[uint64]*listenInfo
	listenLock sync.RWMutex

	// Signal to shutdown dht
	shutdown chan struct{}

	// When this peer started up
	birth time.Time

	//lock to make diagnostics work better
	diaglock sync.Mutex
}

// The listen info struct holds information about a message that is being waited for
type listenInfo struct {
	// Responses matching the listen ID will be sent through resp
	resp chan *swarm.Message

	// count is the number of responses to listen for
	count int

	// eol is the time at which this listener will expire
	eol time.Time
}

// Create a new DHT object with the given peer as the 'local' host
func NewDHT(p *peer.Peer) (*IpfsDHT, error) {
	if p == nil {
		return nil, errors.New("nil peer passed to NewDHT()")
	}
	network := swarm.NewSwarm(p)
	err := network.Listen()
	if err != nil {
		return nil, err
	}

	dht := new(IpfsDHT)
	dht.network = network
	dht.datastore = ds.NewMapDatastore()
	dht.self = p
	dht.listeners = make(map[uint64]*listenInfo)
	dht.providers = make(map[u.Key][]*providerInfo)
	dht.shutdown = make(chan struct{})
	dht.routes = make([]*kb.RoutingTable, 1)
	dht.routes[0] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID))
	dht.birth = time.Now()
	return dht, nil
}

// Start up background goroutines needed by the DHT
func (dht *IpfsDHT) Start() {
	go dht.handleMessages()
}

// Connect to a new peer at the given address
func (dht *IpfsDHT) Connect(addr *ma.Multiaddr) (*peer.Peer, error) {
	maddrstr, _ := addr.String()
	u.DOut("Connect to new peer: %s", maddrstr)
	npeer, err := dht.network.Connect(addr)
	if err != nil {
		return nil, err
	}

	dht.Update(npeer)

	// Ping new peer to register in their routing table
	// NOTE: this should be done better...
	err = dht.Ping(npeer, time.Second*2)
	if err != nil {
		return nil, errors.New("Failed to ping newly connected peer.")
	}

	return npeer, nil
}

// Read in all messages from swarm and handle them appropriately
// NOTE: this function is just a quick sketch
func (dht *IpfsDHT) handleMessages() {
	u.DOut("Begin message handling routine")

	checkTimeouts := time.NewTicker(time.Minute * 5)
	for {
		select {
		case mes, ok := <-dht.network.Chan.Incoming:
			if !ok {
				u.DOut("handleMessages closing, bad recv on incoming")
				return
			}
			pmes := new(PBDHTMessage)
			err := proto.Unmarshal(mes.Data, pmes)
			if err != nil {
				u.PErr("Failed to decode protobuf message: %s", err)
				continue
			}

			dht.Update(mes.Peer)

			// Note: not sure if this is the correct place for this
			if pmes.GetResponse() {
				dht.listenLock.RLock()
				list, ok := dht.listeners[pmes.GetId()]
				dht.listenLock.RUnlock()
				if time.Now().After(list.eol) {
					dht.Unlisten(pmes.GetId())
					ok = false
				}
				if list.count > 1 {
					list.count--
				}
				if ok {
					list.resp <- mes
					if list.count == 1 {
						dht.Unlisten(pmes.GetId())
					}
				} else {
					u.DOut("Received response with nobody listening...")
				}

				continue
			}
			//

			u.DOut("[peer: %s]", dht.self.ID.Pretty())
			u.DOut("Got message type: '%s' [id = %x, from = %s]",
				PBDHTMessage_MessageType_name[int32(pmes.GetType())],
				pmes.GetId(), mes.Peer.ID.Pretty())
			switch pmes.GetType() {
			case PBDHTMessage_GET_VALUE:
				dht.handleGetValue(mes.Peer, pmes)
			case PBDHTMessage_PUT_VALUE:
				dht.handlePutValue(mes.Peer, pmes)
			case PBDHTMessage_FIND_NODE:
				dht.handleFindPeer(mes.Peer, pmes)
			case PBDHTMessage_ADD_PROVIDER:
				dht.handleAddProvider(mes.Peer, pmes)
			case PBDHTMessage_GET_PROVIDERS:
				dht.handleGetProviders(mes.Peer, pmes)
			case PBDHTMessage_PING:
				dht.handlePing(mes.Peer, pmes)
			case PBDHTMessage_DIAGNOSTIC:
				dht.handleDiagnostic(mes.Peer, pmes)
			}

		case err := <-dht.network.Chan.Errors:
			u.DErr("dht err: %s", err)
		case <-dht.shutdown:
			checkTimeouts.Stop()
			return
		case <-checkTimeouts.C:
			// Time to collect some garbage!
			dht.cleanExpiredProviders()
			dht.cleanExpiredListeners()
		}
	}
}

func (dht *IpfsDHT) cleanExpiredProviders() {
	dht.providerLock.Lock()
	for k, parr := range dht.providers {
		var cleaned []*providerInfo
		for _, v := range parr {
			if time.Since(v.Creation) < time.Hour {
				cleaned = append(cleaned, v)
			}
		}
		dht.providers[k] = cleaned
	}
	dht.providerLock.Unlock()
}

func (dht *IpfsDHT) cleanExpiredListeners() {
	dht.listenLock.Lock()
	var remove []uint64
	now := time.Now()
	for k, v := range dht.listeners {
		if now.After(v.eol) {
			remove = append(remove, k)
		}
	}
	for _, k := range remove {
		delete(dht.listeners, k)
	}
	dht.listenLock.Unlock()
}

func (dht *IpfsDHT) putValueToPeer(p *peer.Peer, key string, value []byte) error {
	pmes := DHTMessage{
		Type:  PBDHTMessage_PUT_VALUE,
		Key:   key,
		Value: value,
		Id:    GenerateMessageID(),
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())
	dht.network.Chan.Outgoing <- mes
	return nil
}

func (dht *IpfsDHT) handleGetValue(p *peer.Peer, pmes *PBDHTMessage) {
	dskey := ds.NewKey(pmes.GetKey())
	var resp *DHTMessage
	i_val, err := dht.datastore.Get(dskey)
	if err == nil {
		resp = &DHTMessage{
			Response: true,
			Id:       *pmes.Id,
			Key:      *pmes.Key,
			Value:    i_val.([]byte),
			Success:  true,
		}
	} else if err == ds.ErrNotFound {
		// Find closest peer(s) to desired key and reply with that info
		closer := dht.routes[0].NearestPeer(kb.ConvertKey(u.Key(pmes.GetKey())))
		resp = &DHTMessage{
			Response: true,
			Id:       *pmes.Id,
			Key:      *pmes.Key,
			Value:    closer.ID,
			Success:  false,
		}
	}

	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.network.Chan.Outgoing <- mes
}

// Store a value in this peer local storage
func (dht *IpfsDHT) handlePutValue(p *peer.Peer, pmes *PBDHTMessage) {
	dskey := ds.NewKey(pmes.GetKey())
	err := dht.datastore.Put(dskey, pmes.GetValue())
	if err != nil {
		// For now, just panic, handle this better later maybe
		panic(err)
	}
}

func (dht *IpfsDHT) handlePing(p *peer.Peer, pmes *PBDHTMessage) {
	resp := DHTMessage{
		Type:     pmes.GetType(),
		Response: true,
		Id:       pmes.GetId(),
	}

	dht.network.Chan.Outgoing <- swarm.NewMessage(p, resp.ToProtobuf())
}

func (dht *IpfsDHT) handleFindPeer(p *peer.Peer, pmes *PBDHTMessage) {
	success := true
	u.POut("handleFindPeer: searching for '%s'", peer.ID(pmes.GetKey()).Pretty())
	closest := dht.routes[0].NearestPeer(kb.ConvertKey(u.Key(pmes.GetKey())))
	if closest == nil {
		u.PErr("handleFindPeer: could not find anything.")
		success = false
	}

	if len(closest.Addresses) == 0 {
		u.PErr("handleFindPeer: no addresses for connected peer...")
		success = false
	}

	u.POut("handleFindPeer: sending back '%s'", closest.ID.Pretty())

	addr, err := closest.Addresses[0].String()
	if err != nil {
		u.PErr(err.Error())
		success = false
	}

	resp := DHTMessage{
		Type:     pmes.GetType(),
		Response: true,
		Id:       pmes.GetId(),
		Value:    []byte(addr),
		Success:  success,
	}

	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.network.Chan.Outgoing <- mes
}

func (dht *IpfsDHT) handleGetProviders(p *peer.Peer, pmes *PBDHTMessage) {
	dht.providerLock.RLock()
	providers := dht.providers[u.Key(pmes.GetKey())]
	dht.providerLock.RUnlock()
	if providers == nil || len(providers) == 0 {
		// ?????
		u.DOut("No known providers for requested key.")
	}

	// This is just a quick hack, formalize method of sending addrs later
	addrs := make(map[u.Key]string)
	for _, prov := range providers {
		ma := prov.Value.NetAddress("tcp")
		str, err := ma.String()
		if err != nil {
			u.PErr("Error: %s", err)
			continue
		}

		addrs[prov.Value.Key()] = str
	}

	success := true
	data, err := json.Marshal(addrs)
	if err != nil {
		u.POut("handleGetProviders: error marshalling struct to JSON: %s", err)
		data = nil
		success = false
	}

	resp := DHTMessage{
		Type:     PBDHTMessage_GET_PROVIDERS,
		Key:      pmes.GetKey(),
		Value:    data,
		Id:       pmes.GetId(),
		Response: true,
		Success:  success,
	}

	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.network.Chan.Outgoing <- mes
}

type providerInfo struct {
	Creation time.Time
	Value    *peer.Peer
}

func (dht *IpfsDHT) handleAddProvider(p *peer.Peer, pmes *PBDHTMessage) {
	//TODO: need to implement TTLs on providers
	key := u.Key(pmes.GetKey())
	dht.addProviderEntry(key, p)
}

// Register a handler for a specific message ID, used for getting replies
// to certain messages (i.e. response to a GET_VALUE message)
func (dht *IpfsDHT) ListenFor(mesid uint64, count int, timeout time.Duration) <-chan *swarm.Message {
	lchan := make(chan *swarm.Message)
	dht.listenLock.Lock()
	dht.listeners[mesid] = &listenInfo{lchan, count, time.Now().Add(timeout)}
	dht.listenLock.Unlock()
	return lchan
}

// Unregister the given message id from the listener map
func (dht *IpfsDHT) Unlisten(mesid uint64) {
	dht.listenLock.Lock()
	list, ok := dht.listeners[mesid]
	if ok {
		delete(dht.listeners, mesid)
	}
	dht.listenLock.Unlock()
	close(list.resp)
}

func (dht *IpfsDHT) IsListening(mesid uint64) bool {
	dht.listenLock.RLock()
	li, ok := dht.listeners[mesid]
	dht.listenLock.RUnlock()
	if time.Now().After(li.eol) {
		dht.listenLock.Lock()
		delete(dht.listeners, mesid)
		dht.listenLock.Unlock()
		return false
	}
	return ok
}

// Stop all communications from this peer and shut down
func (dht *IpfsDHT) Halt() {
	dht.shutdown <- struct{}{}
	dht.network.Close()
}

func (dht *IpfsDHT) addProviderEntry(key u.Key, p *peer.Peer) {
	u.DOut("Adding %s as provider for '%s'", p.Key().Pretty(), key)
	dht.providerLock.Lock()
	provs := dht.providers[key]
	dht.providers[key] = append(provs, &providerInfo{time.Now(), p})
	dht.providerLock.Unlock()
}

func (dht *IpfsDHT) handleDiagnostic(p *peer.Peer, pmes *PBDHTMessage) {
	dht.diaglock.Lock()
	if dht.IsListening(pmes.GetId()) {
		//TODO: ehhh..........
		dht.diaglock.Unlock()
		return
	}
	dht.diaglock.Unlock()

	seq := dht.routes[0].NearestPeers(kb.ConvertPeerID(dht.self.ID), 10)
	listen_chan := dht.ListenFor(pmes.GetId(), len(seq), time.Second*30)

	for _, ps := range seq {
		mes := swarm.NewMessage(ps, pmes)
		dht.network.Chan.Outgoing <- mes
	}

	buf := new(bytes.Buffer)
	di := dht.getDiagInfo()
	buf.Write(di.Marshal())

	// NOTE: this shouldnt be a hardcoded value
	after := time.After(time.Second * 20)
	count := len(seq)
	for count > 0 {
		select {
		case <-after:
			//Timeout, return what we have
			goto out
		case req_resp := <-listen_chan:
			pmes_out := new(PBDHTMessage)
			err := proto.Unmarshal(req_resp.Data, pmes_out)
			if err != nil {
				// It broke? eh, whatever, keep going
				continue
			}
			buf.Write(req_resp.Data)
			count--
		}
	}

out:
	resp := DHTMessage{
		Type:     PBDHTMessage_DIAGNOSTIC,
		Id:       pmes.GetId(),
		Value:    buf.Bytes(),
		Response: true,
	}

	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.network.Chan.Outgoing <- mes
}

func (dht *IpfsDHT) GetLocal(key u.Key) ([]byte, error) {
	v, err := dht.datastore.Get(ds.NewKey(string(key)))
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

func (dht *IpfsDHT) PutLocal(key u.Key, value []byte) error {
	return dht.datastore.Put(ds.NewKey(string(key)), value)
}

func (dht *IpfsDHT) Update(p *peer.Peer) {
	removed := dht.routes[0].Update(p)
	if removed != nil {
		dht.network.Drop(removed)
	}
}
