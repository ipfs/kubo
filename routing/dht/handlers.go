package dht

import (
	"errors"
	"fmt"
	"time"

	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
)

// dhthandler specifies the signature of functions that handle DHT messages.
type dhtHandler func(*peer.Peer, *Message) (*Message, error)

func (dht *IpfsDHT) handlerForMsgType(t Message_MessageType) dhtHandler {
	switch t {
	case Message_GET_VALUE:
		return dht.handleGetValue
	case Message_PUT_VALUE:
		return dht.handlePutValue
	case Message_FIND_NODE:
		return dht.handleFindPeer
	case Message_ADD_PROVIDER:
		return dht.handleAddProvider
	case Message_GET_PROVIDERS:
		return dht.handleGetProviders
	case Message_PING:
		return dht.handlePing
	case Message_DIAGNOSTIC:
		return dht.handleDiagnostic
	default:
		return nil
	}
}

func (dht *IpfsDHT) putValueToNetwork(p *peer.Peer, key string, value []byte) error {
	typ := Message_PUT_VALUE
	pmes := &Message{
		Type:  &typ,
		Key:   &key,
		Value: value,
	}

	mes, err := msg.FromObject(p, pmes)
	if err != nil {
		return err
	}
	return dht.sender.SendMessage(context.TODO(), mes)
}

func (dht *IpfsDHT) handleGetValue(p *peer.Peer, pmes *Message) (*Message, error) {
	u.DOut("handleGetValue for key: %s\n", pmes.GetKey())

	// setup response
	resp := &Message{
		Type: pmes.Type,
		Key:  pmes.Key,
	}

	// first, is the key even a key?
	key := pmes.GetKey()
	if key == "" {
		return nil, errors.New("handleGetValue but no key was provided")
	}

	// let's first check if we have the value locally.
	dskey := ds.NewKey(pmes.GetKey())
	iVal, err := dht.datastore.Get(dskey)

	// if we got an unexpected error, bail.
	if err != ds.ErrNotFound {
		return nil, err
	}

	// if we have the value, respond with it!
	if err == nil {
		u.DOut("handleGetValue success!\n")

		byts, ok := iVal.([]byte)
		if !ok {
			return nil, fmt.Errorf("datastore had non byte-slice value for %v", dskey)
		}

		resp.Value = byts
		return resp, nil
	}

	// if we know any providers for the requested value, return those.
	provs := dht.providers.GetProviders(u.Key(pmes.GetKey()))
	if len(provs) > 0 {
		u.DOut("handleGetValue returning %d provider[s]\n", len(provs))
		resp.ProviderPeers = peersToPBPeers(provs)
		return resp, nil
	}

	// Find closest peer on given cluster to desired key and reply with that info
	closer := dht.betterPeerToQuery(pmes)
	if closer == nil {
		u.DOut("handleGetValue could not find a closer node than myself.\n")
		resp.CloserPeers = nil
		return resp, nil
	}

	// we got a closer peer, it seems. return it.
	u.DOut("handleGetValue returning a closer peer: '%s'\n", closer.ID.Pretty())
	resp.CloserPeers = peersToPBPeers([]*peer.Peer{closer})
	return resp, nil
}

// Store a value in this peer local storage
func (dht *IpfsDHT) handlePutValue(p *peer.Peer, pmes *Message) {
	dht.dslock.Lock()
	defer dht.dslock.Unlock()
	dskey := ds.NewKey(pmes.GetKey())
	err := dht.datastore.Put(dskey, pmes.GetValue())
	if err != nil {
		// For now, just panic, handle this better later maybe
		panic(err)
	}
}

func (dht *IpfsDHT) handlePing(p *peer.Peer, pmes *Message) (*Message, error) {
	u.DOut("[%s] Responding to ping from [%s]!\n", dht.self.ID.Pretty(), p.ID.Pretty())
	return &Message{Type: pmes.Type}, nil
}

func (dht *IpfsDHT) handleFindPeer(p *peer.Peer, pmes *Message) (*Message, error) {
	resp := &Message{Type: pmes.Type}
	var closest *peer.Peer

	// if looking for self... special case where we send it on CloserPeers.
	if peer.ID(pmes.GetKey()).Equal(dht.self.ID) {
		closest = dht.self
	} else {
		closest = dht.betterPeerToQuery(pmes)
	}

	if closest == nil {
		u.PErr("handleFindPeer: could not find anything.\n")
		return resp, nil
	}

	if len(closest.Addresses) == 0 {
		u.PErr("handleFindPeer: no addresses for connected peer...\n")
		return resp, nil
	}

	u.DOut("handleFindPeer: sending back '%s'\n", closest.ID.Pretty())
	resp.CloserPeers = peersToPBPeers([]*peer.Peer{closest})
	return resp, nil
}

func (dht *IpfsDHT) handleGetProviders(p *peer.Peer, pmes *Message) (*Message, error) {
	resp := &Message{
		Type: pmes.Type,
		Key:  pmes.Key,
	}

	// check if we have this value, to add ourselves as provider.
	has, err := dht.datastore.Has(ds.NewKey(pmes.GetKey()))
	if err != nil && err != ds.ErrNotFound {
		u.PErr("unexpected datastore error: %v\n", err)
		has = false
	}

	// setup providers
	providers := dht.providers.GetProviders(u.Key(pmes.GetKey()))
	if has {
		providers = append(providers, dht.self)
	}

	// if we've got providers, send thos those.
	if providers != nil && len(providers) > 0 {
		resp.ProviderPeers = peersToPBPeers(providers)
	}

	// Also send closer peers.
	closer := dht.betterPeerToQuery(pmes)
	if closer != nil {
		resp.CloserPeers = peersToPBPeers([]*peer.Peer{closer})
	}

	return resp, nil
}

type providerInfo struct {
	Creation time.Time
	Value    *peer.Peer
}

func (dht *IpfsDHT) handleAddProvider(p *peer.Peer, pmes *Message) {
	key := u.Key(pmes.GetKey())
	u.DOut("[%s] Adding [%s] as a provider for '%s'\n",
		dht.self.ID.Pretty(), p.ID.Pretty(), peer.ID(key).Pretty())
	dht.providers.AddProvider(key, p)
}

// Halt stops all communications from this peer and shut down
// TODO -- remove this in favor of context
func (dht *IpfsDHT) Halt() {
	dht.shutdown <- struct{}{}
	dht.network.Close()
	dht.providers.Halt()
}

// NOTE: not yet finished, low priority
func (dht *IpfsDHT) handleDiagnostic(p *peer.Peer, pmes *Message) (*Message, error) {
	seq := dht.routingTables[0].NearestPeers(kb.ConvertPeerID(dht.self.ID), 10)

	for _, ps := range seq {
		mes, err := msg.FromObject(ps, pmes)
		if err != nil {
			u.PErr("handleDiagnostics error creating message: %v\n", err)
			continue
		}
		// dht.sender.SendRequest(context.TODO(), mes)
	}
	return nil, errors.New("not yet ported back")

	// 	buf := new(bytes.Buffer)
	// 	di := dht.getDiagInfo()
	// 	buf.Write(di.Marshal())
	//
	// 	// NOTE: this shouldnt be a hardcoded value
	// 	after := time.After(time.Second * 20)
	// 	count := len(seq)
	// 	for count > 0 {
	// 		select {
	// 		case <-after:
	// 			//Timeout, return what we have
	// 			goto out
	// 		case reqResp := <-listenChan:
	// 			pmesOut := new(Message)
	// 			err := proto.Unmarshal(reqResp.Data, pmesOut)
	// 			if err != nil {
	// 				// It broke? eh, whatever, keep going
	// 				continue
	// 			}
	// 			buf.Write(reqResp.Data)
	// 			count--
	// 		}
	// 	}
	//
	// out:
	// 	resp := Message{
	// 		Type:     Message_DIAGNOSTIC,
	// 		ID:       pmes.GetId(),
	// 		Value:    buf.Bytes(),
	// 		Response: true,
	// 	}
	//
	// 	mes := swarm.NewMessage(p, resp.ToProtobuf())
	// 	dht.netChan.Outgoing <- mes
}
