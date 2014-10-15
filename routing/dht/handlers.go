package dht

import (
	"errors"
	"fmt"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
)

var CloserPeerCount = 4

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
	default:
		return nil
	}
}

func (dht *IpfsDHT) handleGetValue(p *peer.Peer, pmes *Message) (*Message, error) {
	log.Debug("%s handleGetValue for key: %s\n", dht.self, pmes.GetKey())

	// setup response
	resp := newMessage(pmes.GetType(), pmes.GetKey(), pmes.GetClusterLevel())

	// first, is the key even a key?
	key := pmes.GetKey()
	if key == "" {
		return nil, errors.New("handleGetValue but no key was provided")
	}

	// let's first check if we have the value locally.
	log.Debug("%s handleGetValue looking into ds\n", dht.self)
	dskey := u.Key(pmes.GetKey()).DsKey()
	iVal, err := dht.datastore.Get(dskey)
	log.Debug("%s handleGetValue looking into ds GOT %v\n", dht.self, iVal)

	// if we got an unexpected error, bail.
	if err != nil && err != ds.ErrNotFound {
		return nil, err
	}

	// Note: changed the behavior here to return _as much_ info as possible
	// (potentially all of {value, closer peers, provider})

	// if we have the value, send it back
	if err == nil {
		log.Debug("%s handleGetValue success!\n", dht.self)

		byts, ok := iVal.([]byte)
		if !ok {
			return nil, fmt.Errorf("datastore had non byte-slice value for %v", dskey)
		}

		resp.Value = byts
	}

	// if we know any providers for the requested value, return those.
	provs := dht.providers.GetProviders(u.Key(pmes.GetKey()))
	if len(provs) > 0 {
		log.Debug("handleGetValue returning %d provider[s]\n", len(provs))
		resp.ProviderPeers = peersToPBPeers(provs)
	}

	// Find closest peer on given cluster to desired key and reply with that info
	closer := dht.betterPeersToQuery(pmes, CloserPeerCount)
	if closer != nil {
		for _, p := range closer {
			log.Debug("handleGetValue returning closer peer: '%s'", p)
		}
		resp.CloserPeers = peersToPBPeers(closer)
	}

	return resp, nil
}

// Store a value in this peer local storage
func (dht *IpfsDHT) handlePutValue(p *peer.Peer, pmes *Message) (*Message, error) {
	dht.dslock.Lock()
	defer dht.dslock.Unlock()
	dskey := u.Key(pmes.GetKey()).DsKey()
	err := dht.datastore.Put(dskey, pmes.GetValue())
	log.Debug("%s handlePutValue %v %v\n", dht.self, dskey, pmes.GetValue())
	return pmes, err
}

func (dht *IpfsDHT) handlePing(p *peer.Peer, pmes *Message) (*Message, error) {
	log.Debug("%s Responding to ping from %s!\n", dht.self, p)
	return pmes, nil
}

func (dht *IpfsDHT) handleFindPeer(p *peer.Peer, pmes *Message) (*Message, error) {
	resp := newMessage(pmes.GetType(), "", pmes.GetClusterLevel())
	var closest []*peer.Peer

	// if looking for self... special case where we send it on CloserPeers.
	if peer.ID(pmes.GetKey()).Equal(dht.self.ID) {
		closest = []*peer.Peer{dht.self}
	} else {
		closest = dht.betterPeersToQuery(pmes, CloserPeerCount)
	}

	if closest == nil {
		log.Error("handleFindPeer: could not find anything.")
		return resp, nil
	}

	var withAddresses []*peer.Peer
	for _, p := range closest {
		if len(p.Addresses) > 0 {
			withAddresses = append(withAddresses, p)
		}
	}

	for _, p := range withAddresses {
		log.Debug("handleFindPeer: sending back '%s'", p)
	}
	resp.CloserPeers = peersToPBPeers(withAddresses)
	return resp, nil
}

func (dht *IpfsDHT) handleGetProviders(p *peer.Peer, pmes *Message) (*Message, error) {
	resp := newMessage(pmes.GetType(), pmes.GetKey(), pmes.GetClusterLevel())

	// check if we have this value, to add ourselves as provider.
	log.Debug("handling GetProviders: '%s'", pmes.GetKey())
	dsk := u.Key(pmes.GetKey()).DsKey()
	has, err := dht.datastore.Has(dsk)
	if err != nil && err != ds.ErrNotFound {
		log.Error("unexpected datastore error: %v\n", err)
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
	closer := dht.betterPeersToQuery(pmes, CloserPeerCount)
	if closer != nil {
		resp.CloserPeers = peersToPBPeers(closer)
	}

	return resp, nil
}

type providerInfo struct {
	Creation time.Time
	Value    *peer.Peer
}

func (dht *IpfsDHT) handleAddProvider(p *peer.Peer, pmes *Message) (*Message, error) {
	key := u.Key(pmes.GetKey())

	log.Debug("%s adding %s as a provider for '%s'\n", dht.self, p, peer.ID(key))

	// add provider should use the address given in the message
	for _, pb := range pmes.GetProviderPeers() {
		pid := peer.ID(pb.GetId())
		if pid.Equal(p.ID) {

			addr, err := pb.Address()
			if err != nil {
				log.Error("provider %s error with address %s", p, *pb.Addr)
				continue
			}

			log.Info("received provider %s %s for %s", p, addr, key)
			p.AddAddress(addr)
			dht.providers.AddProvider(key, p)

		} else {
			log.Error("handleAddProvider received provider %s from %s", pid, p)
		}
	}

	return pmes, nil // send back same msg as confirmation.
}

// Halt stops all communications from this peer and shut down
// TODO -- remove this in favor of context
func (dht *IpfsDHT) Halt() {
	dht.providers.Halt()
}
