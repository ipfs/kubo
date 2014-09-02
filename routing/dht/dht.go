package dht

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"

	ma "github.com/jbenet/go-multiaddr"

	ds "github.com/jbenet/datastore.go"

	context "code.google.com/p/go.net/context"

	"code.google.com/p/goprotobuf/proto"
)

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	// Array of routing tables for differently distanced nodes
	// NOTE: (currently, only a single table is used)
	routingTables []*kb.RoutingTable

	network swarm.Network
	netChan *swarm.Chan

	// Local peer (yourself)
	self *peer.Peer

	// Local data
	datastore ds.Datastore
	dslock    sync.Mutex

	providers *ProviderManager

	// Signal to shutdown dht
	shutdown chan struct{}

	// When this peer started up
	birth time.Time

	//lock to make diagnostics work better
	diaglock sync.Mutex

	// listener is a server to register to listen for responses to messages
	listener *swarm.MessageListener
}

// NewDHT creates a new DHT object with the given peer as the 'local' host
func NewDHT(p *peer.Peer, net swarm.Network, dstore ds.Datastore) *IpfsDHT {
	dht := new(IpfsDHT)
	dht.network = net
	dht.netChan = net.GetChannel(swarm.PBWrapper_DHT_MESSAGE)
	dht.datastore = dstore
	dht.self = p
	dht.providers = NewProviderManager()
	dht.shutdown = make(chan struct{})

	dht.routingTables = make([]*kb.RoutingTable, 3)
	dht.routingTables[0] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID), time.Millisecond*30)
	dht.routingTables[1] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID), time.Millisecond*100)
	dht.routingTables[2] = kb.NewRoutingTable(20, kb.ConvertPeerID(p.ID), time.Hour)
	dht.listener = swarm.NewMessageListener()
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
	u.DOut("Connect to new peer: %s\n", maddrstr)
	npeer, err := dht.network.ConnectNew(addr)
	if err != nil {
		return nil, err
	}

	// Ping new peer to register in their routing table
	// NOTE: this should be done better...
	err = dht.Ping(npeer, time.Second*2)
	if err != nil {
		return nil, fmt.Errorf("failed to ping newly connected peer: %s\n", err)
	}

	dht.Update(npeer)

	return npeer, nil
}

// Read in all messages from swarm and handle them appropriately
// NOTE: this function is just a quick sketch
func (dht *IpfsDHT) handleMessages() {
	u.DOut("Begin message handling routine\n")

	errs := dht.network.GetErrChan()
	for {
		select {
		case mes, ok := <-dht.netChan.Incoming:
			if !ok {
				u.DOut("handleMessages closing, bad recv on incoming\n")
				return
			}
			pmes := new(PBDHTMessage)
			err := proto.Unmarshal(mes.Data, pmes)
			if err != nil {
				u.PErr("Failed to decode protobuf message: %s\n", err)
				continue
			}

			dht.Update(mes.Peer)

			// Note: not sure if this is the correct place for this
			if pmes.GetResponse() {
				dht.listener.Respond(pmes.GetId(), mes)
				continue
			}
			//

			u.DOut("[peer: %s]\nGot message type: '%s' [id = %x, from = %s]\n",
				dht.self.ID.Pretty(),
				PBDHTMessage_MessageType_name[int32(pmes.GetType())],
				pmes.GetId(), mes.Peer.ID.Pretty())
			switch pmes.GetType() {
			case PBDHTMessage_GET_VALUE:
				go dht.handleGetValue(mes.Peer, pmes)
			case PBDHTMessage_PUT_VALUE:
				go dht.handlePutValue(mes.Peer, pmes)
			case PBDHTMessage_FIND_NODE:
				go dht.handleFindPeer(mes.Peer, pmes)
			case PBDHTMessage_ADD_PROVIDER:
				go dht.handleAddProvider(mes.Peer, pmes)
			case PBDHTMessage_GET_PROVIDERS:
				go dht.handleGetProviders(mes.Peer, pmes)
			case PBDHTMessage_PING:
				go dht.handlePing(mes.Peer, pmes)
			case PBDHTMessage_DIAGNOSTIC:
				go dht.handleDiagnostic(mes.Peer, pmes)
			default:
				u.PErr("Recieved invalid message type")
			}

		case err := <-errs:
			u.PErr("dht err: %s\n", err)
		case <-dht.shutdown:
			return
		}
	}
}

func (dht *IpfsDHT) putValueToNetwork(p *peer.Peer, key string, value []byte) error {
	pmes := Message{
		Type:  PBDHTMessage_PUT_VALUE,
		Key:   key,
		Value: value,
		ID:    swarm.GenerateMessageID(),
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())
	dht.netChan.Outgoing <- mes
	return nil
}

func (dht *IpfsDHT) handleGetValue(p *peer.Peer, pmes *PBDHTMessage) {
	u.DOut("handleGetValue for key: %s\n", pmes.GetKey())
	dskey := ds.NewKey(pmes.GetKey())
	resp := &Message{
		Response: true,
		ID:       pmes.GetId(),
		Key:      pmes.GetKey(),
	}
	iVal, err := dht.datastore.Get(dskey)
	if err == nil {
		u.DOut("handleGetValue success!\n")
		resp.Success = true
		resp.Value = iVal.([]byte)
	} else if err == ds.ErrNotFound {
		// Check if we know any providers for the requested value
		provs := dht.providers.GetProviders(u.Key(pmes.GetKey()))
		if len(provs) > 0 {
			u.DOut("handleGetValue returning %d provider[s]\n", len(provs))
			resp.Peers = provs
			resp.Success = true
		} else {
			// No providers?
			// Find closest peer on given cluster to desired key and reply with that info

			level := 0
			if len(pmes.GetValue()) < 1 {
				// TODO: maybe return an error? Defaulting isnt a good idea IMO
				u.PErr("handleGetValue: no routing level specified, assuming 0\n")
			} else {
				level = int(pmes.GetValue()[0]) // Using value field to specify cluster level
			}
			u.DOut("handleGetValue searching level %d clusters\n", level)

			closer := dht.routingTables[level].NearestPeer(kb.ConvertKey(u.Key(pmes.GetKey())))

			if closer.ID.Equal(dht.self.ID) {
				u.DOut("Attempted to return self! this shouldnt happen...\n")
				resp.Peers = nil
				goto out
			}
			// If this peer is closer than the one from the table, return nil
			if kb.Closer(dht.self.ID, closer.ID, u.Key(pmes.GetKey())) {
				resp.Peers = nil
				u.DOut("handleGetValue could not find a closer node than myself.\n")
			} else {
				u.DOut("handleGetValue returning a closer peer: '%s'\n", closer.ID.Pretty())
				resp.Peers = []*peer.Peer{closer}
			}
		}
	} else {
		//temp: what other errors can a datastore return?
		panic(err)
	}

out:
	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.netChan.Outgoing <- mes
}

// Store a value in this peer local storage
func (dht *IpfsDHT) handlePutValue(p *peer.Peer, pmes *PBDHTMessage) {
	dht.dslock.Lock()
	defer dht.dslock.Unlock()
	dskey := ds.NewKey(pmes.GetKey())
	err := dht.datastore.Put(dskey, pmes.GetValue())
	if err != nil {
		// For now, just panic, handle this better later maybe
		panic(err)
	}
}

func (dht *IpfsDHT) handlePing(p *peer.Peer, pmes *PBDHTMessage) {
	u.DOut("[%s] Responding to ping from [%s]!\n", dht.self.ID.Pretty(), p.ID.Pretty())
	resp := Message{
		Type:     pmes.GetType(),
		Response: true,
		ID:       pmes.GetId(),
	}

	dht.netChan.Outgoing <- swarm.NewMessage(p, resp.ToProtobuf())
}

func (dht *IpfsDHT) handleFindPeer(p *peer.Peer, pmes *PBDHTMessage) {
	resp := Message{
		Type:     pmes.GetType(),
		ID:       pmes.GetId(),
		Response: true,
	}
	defer func() {
		mes := swarm.NewMessage(p, resp.ToProtobuf())
		dht.netChan.Outgoing <- mes
	}()
	level := pmes.GetValue()[0]
	u.DOut("handleFindPeer: searching for '%s'\n", peer.ID(pmes.GetKey()).Pretty())
	closest := dht.routingTables[level].NearestPeer(kb.ConvertKey(u.Key(pmes.GetKey())))
	if closest == nil {
		u.PErr("handleFindPeer: could not find anything.\n")
		return
	}

	if len(closest.Addresses) == 0 {
		u.PErr("handleFindPeer: no addresses for connected peer...\n")
		return
	}

	// If the found peer further away than this peer...
	if kb.Closer(dht.self.ID, closest.ID, u.Key(pmes.GetKey())) {
		return
	}

	u.DOut("handleFindPeer: sending back '%s'\n", closest.ID.Pretty())
	resp.Peers = []*peer.Peer{closest}
	resp.Success = true
}

func (dht *IpfsDHT) handleGetProviders(p *peer.Peer, pmes *PBDHTMessage) {
	resp := Message{
		Type:     PBDHTMessage_GET_PROVIDERS,
		Key:      pmes.GetKey(),
		ID:       pmes.GetId(),
		Response: true,
	}

	providers := dht.providers.GetProviders(u.Key(pmes.GetKey()))
	if providers == nil || len(providers) == 0 {
		level := 0
		if len(pmes.GetValue()) > 0 {
			level = int(pmes.GetValue()[0])
		}

		closer := dht.routingTables[level].NearestPeer(kb.ConvertKey(u.Key(pmes.GetKey())))
		if kb.Closer(dht.self.ID, closer.ID, u.Key(pmes.GetKey())) {
			resp.Peers = nil
		} else {
			resp.Peers = []*peer.Peer{closer}
		}
	} else {
		resp.Peers = providers
		resp.Success = true
	}

	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.netChan.Outgoing <- mes
}

type providerInfo struct {
	Creation time.Time
	Value    *peer.Peer
}

func (dht *IpfsDHT) handleAddProvider(p *peer.Peer, pmes *PBDHTMessage) {
	key := u.Key(pmes.GetKey())
	u.DOut("[%s] Adding [%s] as a provider for '%s'\n", dht.self.ID.Pretty(), p.ID.Pretty(), peer.ID(key).Pretty())
	dht.providers.AddProvider(key, p)
}

// Halt stops all communications from this peer and shut down
func (dht *IpfsDHT) Halt() {
	dht.shutdown <- struct{}{}
	dht.network.Close()
	dht.providers.Halt()
	dht.listener.Halt()
}

// NOTE: not yet finished, low priority
func (dht *IpfsDHT) handleDiagnostic(p *peer.Peer, pmes *PBDHTMessage) {
	seq := dht.routingTables[0].NearestPeers(kb.ConvertPeerID(dht.self.ID), 10)
	listenChan := dht.listener.Listen(pmes.GetId(), len(seq), time.Second*30)

	for _, ps := range seq {
		mes := swarm.NewMessage(ps, pmes)
		dht.netChan.Outgoing <- mes
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
		case reqResp := <-listenChan:
			pmesOut := new(PBDHTMessage)
			err := proto.Unmarshal(reqResp.Data, pmesOut)
			if err != nil {
				// It broke? eh, whatever, keep going
				continue
			}
			buf.Write(reqResp.Data)
			count--
		}
	}

out:
	resp := Message{
		Type:     PBDHTMessage_DIAGNOSTIC,
		ID:       pmes.GetId(),
		Value:    buf.Bytes(),
		Response: true,
	}

	mes := swarm.NewMessage(p, resp.ToProtobuf())
	dht.netChan.Outgoing <- mes
}

func (dht *IpfsDHT) getValueOrPeers(p *peer.Peer, key u.Key, timeout time.Duration, level int) ([]byte, []*peer.Peer, error) {
	pmes, err := dht.getValueSingle(p, key, timeout, level)
	if err != nil {
		return nil, nil, err
	}

	if pmes.GetSuccess() {
		if pmes.Value == nil { // We were given provider[s]
			val, err := dht.getFromPeerList(key, timeout, pmes.GetPeers(), level)
			if err != nil {
				return nil, nil, err
			}
			return val, nil, nil
		}

		// Success! We were given the value
		return pmes.GetValue(), nil, nil
	}

	// We were given a closer node
	var peers []*peer.Peer
	for _, pb := range pmes.GetPeers() {
		if peer.ID(pb.GetId()).Equal(dht.self.ID) {
			continue
		}
		addr, err := ma.NewMultiaddr(pb.GetAddr())
		if err != nil {
			u.PErr("%v\n", err.Error())
			continue
		}

		np, err := dht.network.GetConnection(peer.ID(pb.GetId()), addr)
		if err != nil {
			u.PErr("%v\n", err.Error())
			continue
		}

		peers = append(peers, np)
	}
	return nil, peers, nil
}

// getValueSingle simply performs the get value RPC with the given parameters
func (dht *IpfsDHT) getValueSingle(p *peer.Peer, key u.Key, timeout time.Duration, level int) (*PBDHTMessage, error) {
	pmes := Message{
		Type:  PBDHTMessage_GET_VALUE,
		Key:   string(key),
		Value: []byte{byte(level)},
		ID:    swarm.GenerateMessageID(),
	}
	responseChan := dht.listener.Listen(pmes.ID, 1, time.Minute)

	mes := swarm.NewMessage(p, pmes.ToProtobuf())
	t := time.Now()
	dht.netChan.Outgoing <- mes

	// Wait for either the response or a timeout
	timeup := time.After(timeout)
	select {
	case <-timeup:
		dht.listener.Unlisten(pmes.ID)
		return nil, u.ErrTimeout
	case resp, ok := <-responseChan:
		if !ok {
			u.PErr("response channel closed before timeout, please investigate.\n")
			return nil, u.ErrTimeout
		}
		roundtrip := time.Since(t)
		resp.Peer.SetLatency(roundtrip)
		pmesOut := new(PBDHTMessage)
		err := proto.Unmarshal(resp.Data, pmesOut)
		if err != nil {
			return nil, err
		}
		return pmesOut, nil
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
				u.PErr("getValue error: %s\n", err)
				continue
			}

			p, err = dht.network.GetConnection(peer.ID(pinfo.GetId()), maddr)
			if err != nil {
				u.PErr("getValue error: %s\n", err)
				continue
			}
		}
		pmes, err := dht.getValueSingle(p, key, timeout, level)
		if err != nil {
			u.DErr("getFromPeers error: %s\n", err)
			continue
		}
		dht.providers.AddProvider(key, p)

		// Make sure it was a successful get
		if pmes.GetSuccess() && pmes.Value != nil {
			return pmes.GetValue(), nil
		}
	}
	return nil, u.ErrNotFound
}

func (dht *IpfsDHT) getLocal(key u.Key) ([]byte, error) {
	dht.dslock.Lock()
	defer dht.dslock.Unlock()
	v, err := dht.datastore.Get(ds.NewKey(string(key)))
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

func (dht *IpfsDHT) putLocal(key u.Key, value []byte) error {
	return dht.datastore.Put(ds.NewKey(string(key)), value)
}

// Update TODO(chas) Document this function
func (dht *IpfsDHT) Update(p *peer.Peer) {
	for _, route := range dht.routingTables {
		removed := route.Update(p)
		// Only drop the connection if no tables refer to this peer
		if removed != nil {
			found := false
			for _, r := range dht.routingTables {
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

// Find looks for a peer with a given ID connected to this dht and returns the peer and the table it was found in.
func (dht *IpfsDHT) Find(id peer.ID) (*peer.Peer, *kb.RoutingTable) {
	for _, table := range dht.routingTables {
		p := table.Find(id)
		if p != nil {
			return p, table
		}
	}
	return nil, nil
}

func (dht *IpfsDHT) findPeerSingle(p *peer.Peer, id peer.ID, timeout time.Duration, level int) (*PBDHTMessage, error) {
	pmes := Message{
		Type:  PBDHTMessage_FIND_NODE,
		Key:   string(id),
		ID:    swarm.GenerateMessageID(),
		Value: []byte{byte(level)},
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())
	listenChan := dht.listener.Listen(pmes.ID, 1, time.Minute)
	t := time.Now()
	dht.netChan.Outgoing <- mes
	after := time.After(timeout)
	select {
	case <-after:
		dht.listener.Unlisten(pmes.ID)
		return nil, u.ErrTimeout
	case resp := <-listenChan:
		roundtrip := time.Since(t)
		resp.Peer.SetLatency(roundtrip)
		pmesOut := new(PBDHTMessage)
		err := proto.Unmarshal(resp.Data, pmesOut)
		if err != nil {
			return nil, err
		}

		return pmesOut, nil
	}
}

func (dht *IpfsDHT) printTables() {
	for _, route := range dht.routingTables {
		route.Print()
	}
}

func (dht *IpfsDHT) findProvidersSingle(ctx context.Context, p *peer.Peer, key u.Key, level int) (*PBDHTMessage, error) {
	pmes := Message{
		Type:  PBDHTMessage_GET_PROVIDERS,
		Key:   string(key),
		ID:    swarm.GenerateMessageID(),
		Value: []byte{byte(level)},
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())

	listenChan := dht.listener.Listen(pmes.ID, 1, time.Minute)
	dht.netChan.Outgoing <- mes
	select {
	case <-ctx.Done():
		dht.listener.Unlisten(pmes.ID)
		return nil, ctx.Err()
	case resp := <-listenChan:
		u.DOut("FindProviders: got response.\n")
		pmesOut := new(PBDHTMessage)
		err := proto.Unmarshal(resp.Data, pmesOut)
		if err != nil {
			return nil, err
		}

		return pmesOut, nil
	}
}

// TODO: Could be done async
func (dht *IpfsDHT) addPeerList(key u.Key, peers []*PBDHTMessage_PBPeer) []*peer.Peer {
	var provArr []*peer.Peer
	for _, prov := range peers {
		// Dont add outselves to the list
		if peer.ID(prov.GetId()).Equal(dht.self.ID) {
			continue
		}
		// Dont add someone who is already on the list
		p := dht.network.Find(u.Key(prov.GetId()))
		if p == nil {
			u.DOut("given provider %s was not in our network already.\n", peer.ID(prov.GetId()).Pretty())
			var err error
			p, err = dht.peerFromInfo(prov)
			if err != nil {
				u.PErr("error connecting to new peer: %s\n", err)
				continue
			}
		}
		dht.providers.AddProvider(key, p)
		provArr = append(provArr, p)
	}
	return provArr
}

func (dht *IpfsDHT) peerFromInfo(pbp *PBDHTMessage_PBPeer) (*peer.Peer, error) {
	maddr, err := ma.NewMultiaddr(pbp.GetAddr())
	if err != nil {
		return nil, err
	}

	return dht.network.GetConnection(peer.ID(pbp.GetId()), maddr)
}
