package dht

import (
	"bytes"
	"errors"
	"sync"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"

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

	network swarm.Network

	// Local peer (yourself)
	self *peer.Peer

	// Local data
	datastore ds.Datastore

	// Map keys to peers that can provide their value
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

// NewDHT creates a new DHT object with the given peer as the 'local' host
func NewDHT(p *peer.Peer, net swarm.Network) *IpfsDHT {
	dht := new(IpfsDHT)
	dht.network = net
	dht.datastore = ds.NewMapDatastore()
	dht.self = p
	dht.listeners = make(map[uint64]*listenInfo)
	dht.providers = make(map[u.Key][]*providerInfo)
	dht.shutdown = make(chan struct{})

	dht.routes = make([]*kb.RoutingTable, 3)
	dht.routes[0] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID), time.Millisecond*30)
	dht.routes[1] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID), time.Millisecond*100)
	dht.routes[2] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID), time.Hour)

	dht.birth = time.Now()
	return dht
}

// Start up background goroutines needed by the DHT
func (dht *IpfsDHT) Start() {
	go dht.handleMessages()
}

// Connect to a new peer at the given address, ping and add to the routing table
func (dht *IpfsDHT) Connect(addr *ma.Multiaddr) (*peer.Peer, error) {
	maddrstr, _ := addr.String()
	u.DOut("Connect to new peer: %s", maddrstr)
	npeer, err := dht.network.ConnectNew(addr)
	if err != nil {
		return nil, err
	}

	// Ping new peer to register in their routing table
	// NOTE: this should be done better...
	err = dht.Ping(npeer, time.Second*2)
	if err != nil {
		return nil, errors.New("failed to ping newly connected peer")
	}

	dht.Update(npeer)

	return npeer, nil
}

// Read in all messages from swarm and handle them appropriately
// NOTE: this function is just a quick sketch
func (dht *IpfsDHT) handleMessages() {
	u.DOut("Begin message handling routine")

	checkTimeouts := time.NewTicker(time.Minute * 5)
	ch := dht.network.GetChan()
	for {
		select {
		case mes, ok := <-ch.Incoming:
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

			u.DOut("[peer: %s]\nGot message type: '%s' [id = %x, from = %s]",
				dht.self.ID.Pretty(),
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

		case err := <-ch.Errors:
			u.PErr("dht err: %s", err)
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

func (dht *IpfsDHT) putValueToNetwork(p *peer.Peer, key string, value []byte) error {
	pmes := DHTMessage{
		Type:  PBDHTMessage_PUT_VALUE,
		Key:   key,
		Value: value,
		Id:    GenerateMessageID(),
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())
	dht.network.Send(mes)
	return nil
}

func (dht *IpfsDHT) handleGetValue(p *peer.Peer, pmes *PBDHTMessage) {
	u.DOut("handleGetValue for key: %s", pmes.GetKey())
	dskey := ds.NewKey(pmes.GetKey())
	resp := &DHTMessage{
		Response: true,
		Id:       pmes.GetId(),
		Key:      pmes.GetKey(),
	}
	iVal, err := dht.datastore.Get(dskey)
	if err == nil {
		u.DOut("handleGetValue success!")
		resp.Success = true
		resp.Value = iVal.([]byte)
	} else if err == ds.ErrNotFound {
		// Check if we know any providers for the requested value
		provs, ok := dht.providers[u.Key(pmes.GetKey())]
		if ok && len(provs) > 0 {
			u.DOut("handleGetValue returning %d provider[s]", len(provs))
			for _, prov := range provs {
				resp.Peers = append(resp.Peers, prov.Value)
			}
			resp.Success = true
		} else {
			// No providers?
			// Find closest peer on given cluster to desired key and reply with that info

			level := 0
			if len(pmes.GetValue()) < 1 {
				// TODO: maybe return an error? Defaulting isnt a good idea IMO
				u.PErr("handleGetValue: no routing level specified, assuming 0")
			} else {
				level = int(pmes.GetValue()[0]) // Using value field to specify cluster level
			}
			u.DOut("handleGetValue searching level %d clusters", level)

			closer := dht.routes[level].NearestPeer(kb.ConvertKey(u.Key(pmes.GetKey())))

			if closer.ID.Equal(dht.self.ID) {
				u.DOut("Attempted to return self! this shouldnt happen...")
				resp.Peers = nil
				goto out
			}
			// If this peer is closer than the one from the table, return nil
			if kb.Closer(dht.self.ID, closer.ID, u.Key(pmes.GetKey())) {
				resp.Peers = nil
				u.DOut("handleGetValue could not find a closer node than myself.")
			} else {
				u.DOut("handleGetValue returning a closer peer: '%s'", closer.ID.Pretty())
				resp.Peers = []*peer.Peer{closer}
			}
		}
	} else {
		//temp: what other errors can a datastore return?
		panic(err)
	}

out:
	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.network.Send(mes)
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

	dht.network.Send(swarm.NewMessage(p, resp.ToProtobuf()))
}

func (dht *IpfsDHT) handleFindPeer(p *peer.Peer, pmes *PBDHTMessage) {
	resp := DHTMessage{
		Type:     pmes.GetType(),
		Id:       pmes.GetId(),
		Response: true,
	}
	defer func() {
		mes := swarm.NewMessage(p, resp.ToProtobuf())
		dht.network.Send(mes)
	}()
	level := pmes.GetValue()[0]
	u.DOut("handleFindPeer: searching for '%s'", peer.ID(pmes.GetKey()).Pretty())
	closest := dht.routes[level].NearestPeer(kb.ConvertKey(u.Key(pmes.GetKey())))
	if closest == nil {
		u.PErr("handleFindPeer: could not find anything.")
		return
	}

	if len(closest.Addresses) == 0 {
		u.PErr("handleFindPeer: no addresses for connected peer...")
		return
	}

	// If the found peer further away than this peer...
	if kb.Closer(dht.self.ID, closest.ID, u.Key(pmes.GetKey())) {
		return
	}

	u.DOut("handleFindPeer: sending back '%s'", closest.ID.Pretty())
	resp.Peers = []*peer.Peer{closest}
	resp.Success = true
}

func (dht *IpfsDHT) handleGetProviders(p *peer.Peer, pmes *PBDHTMessage) {
	resp := DHTMessage{
		Type:     PBDHTMessage_GET_PROVIDERS,
		Key:      pmes.GetKey(),
		Id:       pmes.GetId(),
		Response: true,
	}

	dht.providerLock.RLock()
	providers := dht.providers[u.Key(pmes.GetKey())]
	dht.providerLock.RUnlock()
	if providers == nil || len(providers) == 0 {
		level := 0
		if len(pmes.GetValue()) > 0 {
			level = int(pmes.GetValue()[0])
		}

		closer := dht.routes[level].NearestPeer(kb.ConvertKey(u.Key(pmes.GetKey())))
		if kb.Closer(dht.self.ID, closer.ID, u.Key(pmes.GetKey())) {
			resp.Peers = nil
		} else {
			resp.Peers = []*peer.Peer{closer}
		}
	} else {
		for _, prov := range providers {
			resp.Peers = append(resp.Peers, prov.Value)
		}
		resp.Success = true
	}

	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.network.Send(mes)
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

// Check whether or not the dht is currently listening for mesid
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

// NOTE: not yet finished, low priority
func (dht *IpfsDHT) handleDiagnostic(p *peer.Peer, pmes *PBDHTMessage) {
	dht.diaglock.Lock()
	if dht.IsListening(pmes.GetId()) {
		//TODO: ehhh..........
		dht.diaglock.Unlock()
		return
	}
	dht.diaglock.Unlock()

	seq := dht.routes[0].NearestPeers(kb.ConvertPeerID(dht.self.ID), 10)
	listenChan := dht.ListenFor(pmes.GetId(), len(seq), time.Second*30)

	for _, ps := range seq {
		mes := swarm.NewMessage(ps, pmes)
		dht.network.Send(mes)
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
		case req_resp := <-listenChan:
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
	dht.network.Send(mes)
}

// getValueSingle simply performs the get value RPC with the given parameters
func (dht *IpfsDHT) getValueSingle(p *peer.Peer, key u.Key, timeout time.Duration, level int) (*PBDHTMessage, error) {
	pmes := DHTMessage{
		Type:  PBDHTMessage_GET_VALUE,
		Key:   string(key),
		Value: []byte{byte(level)},
		Id:    GenerateMessageID(),
	}
	response_chan := dht.ListenFor(pmes.Id, 1, time.Minute)

	mes := swarm.NewMessage(p, pmes.ToProtobuf())
	t := time.Now()
	dht.network.Send(mes)

	// Wait for either the response or a timeout
	timeup := time.After(timeout)
	select {
	case <-timeup:
		dht.Unlisten(pmes.Id)
		return nil, u.ErrTimeout
	case resp, ok := <-response_chan:
		if !ok {
			u.PErr("response channel closed before timeout, please investigate.")
			return nil, u.ErrTimeout
		}
		roundtrip := time.Since(t)
		resp.Peer.SetLatency(roundtrip)
		pmes_out := new(PBDHTMessage)
		err := proto.Unmarshal(resp.Data, pmes_out)
		if err != nil {
			return nil, err
		}
		return pmes_out, nil
	}
}

// TODO: Im not certain on this implementation, we get a list of peers/providers
// from someone what do we do with it? Connect to each of them? randomly pick
// one to get the value from? Or just connect to one at a time until we get a
// successful connection and request the value from it?
func (dht *IpfsDHT) getFromPeerList(key u.Key, timeout time.Duration,
	peerlist []*PBDHTMessage_PBPeer, level int) ([]byte, error) {
	for _, pinfo := range peerlist {
		p, _ := dht.Find(peer.ID(pinfo.GetId()))
		if p == nil {
			maddr, err := ma.NewMultiaddr(pinfo.GetAddr())
			if err != nil {
				u.PErr("getValue error: %s", err)
				continue
			}

			p, err = dht.network.GetConnection(peer.ID(pinfo.GetId()), maddr)
			if err != nil {
				u.PErr("getValue error: %s", err)
				continue
			}
		}
		pmes, err := dht.getValueSingle(p, key, timeout, level)
		if err != nil {
			u.DErr("getFromPeers error: %s", err)
			continue
		}
		dht.addProviderEntry(key, p)

		// Make sure it was a successful get
		if pmes.GetSuccess() && pmes.Value != nil {
			return pmes.GetValue(), nil
		}
	}
	return nil, u.ErrNotFound
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
	for _, route := range dht.routes {
		removed := route.Update(p)
		// Only drop the connection if no tables refer to this peer
		if removed != nil {
			found := false
			for _, r := range dht.routes {
				if r.Find(removed.ID) != nil {
					found = true
					break
				}
			}
			if !found {
				dht.network.Drop(removed)
			}
		}
	}
}

// Look for a peer with a given ID connected to this dht
func (dht *IpfsDHT) Find(id peer.ID) (*peer.Peer, *kb.RoutingTable) {
	for _, table := range dht.routes {
		p := table.Find(id)
		if p != nil {
			return p, table
		}
	}
	return nil, nil
}

func (dht *IpfsDHT) findPeerSingle(p *peer.Peer, id peer.ID, timeout time.Duration, level int) (*PBDHTMessage, error) {
	pmes := DHTMessage{
		Type:  PBDHTMessage_FIND_NODE,
		Key:   string(id),
		Id:    GenerateMessageID(),
		Value: []byte{byte(level)},
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())
	listenChan := dht.ListenFor(pmes.Id, 1, time.Minute)
	t := time.Now()
	dht.network.Send(mes)
	after := time.After(timeout)
	select {
	case <-after:
		dht.Unlisten(pmes.Id)
		return nil, u.ErrTimeout
	case resp := <-listenChan:
		roundtrip := time.Since(t)
		resp.Peer.SetLatency(roundtrip)
		pmes_out := new(PBDHTMessage)
		err := proto.Unmarshal(resp.Data, pmes_out)
		if err != nil {
			return nil, err
		}

		return pmes_out, nil
	}
}

func (dht *IpfsDHT) PrintTables() {
	for _, route := range dht.routes {
		route.Print()
	}
}

func (dht *IpfsDHT) findProvidersSingle(p *peer.Peer, key u.Key, level int, timeout time.Duration) (*PBDHTMessage, error) {
	pmes := DHTMessage{
		Type:  PBDHTMessage_GET_PROVIDERS,
		Key:   string(key),
		Id:    GenerateMessageID(),
		Value: []byte{byte(level)},
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())

	listenChan := dht.ListenFor(pmes.Id, 1, time.Minute)
	dht.network.Send(mes)
	after := time.After(timeout)
	select {
	case <-after:
		dht.Unlisten(pmes.Id)
		return nil, u.ErrTimeout
	case resp := <-listenChan:
		u.DOut("FindProviders: got response.")
		pmes_out := new(PBDHTMessage)
		err := proto.Unmarshal(resp.Data, pmes_out)
		if err != nil {
			return nil, err
		}

		return pmes_out, nil
	}
}

func (dht *IpfsDHT) addPeerList(key u.Key, peers []*PBDHTMessage_PBPeer) []*peer.Peer {
	var prov_arr []*peer.Peer
	for _, prov := range peers {
		// Dont add outselves to the list
		if peer.ID(prov.GetId()).Equal(dht.self.ID) {
			continue
		}
		// Dont add someone who is already on the list
		p := dht.network.Find(u.Key(prov.GetId()))
		if p == nil {
			u.DOut("given provider %s was not in our network already.", peer.ID(prov.GetId()).Pretty())
			maddr, err := ma.NewMultiaddr(prov.GetAddr())
			if err != nil {
				u.PErr("error connecting to new peer: %s", err)
				continue
			}
			p, err = dht.network.GetConnection(peer.ID(prov.GetId()), maddr)
			if err != nil {
				u.PErr("error connecting to new peer: %s", err)
				continue
			}
		}
		dht.addProviderEntry(key, p)
		prov_arr = append(prov_arr, p)
	}
	return prov_arr
}
