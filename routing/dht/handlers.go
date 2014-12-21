package dht

import (
	"errors"
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	peer "github.com/jbenet/go-ipfs/peer"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	u "github.com/jbenet/go-ipfs/util"
)

// The number of closer peers to send on requests.
var CloserPeerCount = 4

// dhthandler specifies the signature of functions that handle DHT messages.
type dhtHandler func(context.Context, peer.ID, *pb.Message) (*pb.Message, error)

func (dht *IpfsDHT) handlerForMsgType(t pb.Message_MessageType) dhtHandler {
	switch t {
	case pb.Message_GET_VALUE:
		return dht.handleGetValue
	case pb.Message_PUT_VALUE:
		return dht.handlePutValue
	case pb.Message_FIND_NODE:
		return dht.handleFindPeer
	case pb.Message_ADD_PROVIDER:
		return dht.handleAddProvider
	case pb.Message_GET_PROVIDERS:
		return dht.handleGetProviders
	case pb.Message_PING:
		return dht.handlePing
	default:
		return nil
	}
}

func (dht *IpfsDHT) handleGetValue(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("%s handleGetValue for key: %s\n", dht.self, pmes.GetKey())

	// setup response
	resp := pb.NewMessage(pmes.GetType(), pmes.GetKey(), pmes.GetClusterLevel())

	// first, is there even a key?
	key := pmes.GetKey()
	if key == "" {
		return nil, errors.New("handleGetValue but no key was provided")
		// TODO: send back an error response? could be bad, but the other node's hanging.
	}

	// let's first check if we have the value locally.
	log.Debugf("%s handleGetValue looking into ds", dht.self)
	dskey := u.Key(pmes.GetKey()).DsKey()
	iVal, err := dht.datastore.Get(dskey)
	log.Debugf("%s handleGetValue looking into ds GOT %v", dht.self, iVal)

	// if we got an unexpected error, bail.
	if err != nil && err != ds.ErrNotFound {
		return nil, err
	}

	// Note: changed the behavior here to return _as much_ info as possible
	// (potentially all of {value, closer peers, provider})

	// if we have the value, send it back
	if err == nil {
		log.Debugf("%s handleGetValue success!", dht.self)

		byts, ok := iVal.([]byte)
		if !ok {
			return nil, fmt.Errorf("datastore had non byte-slice value for %v", dskey)
		}

		rec := new(pb.Record)
		err := proto.Unmarshal(byts, rec)
		if err != nil {
			log.Error("Failed to unmarshal dht record from datastore")
			return nil, err
		}

		resp.Record = rec
	}

	// if we know any providers for the requested value, return those.
	provs := dht.providers.GetProviders(ctx, u.Key(pmes.GetKey()))
	provinfos := peer.PeerInfos(dht.peerstore, provs)
	if len(provs) > 0 {
		log.Debugf("handleGetValue returning %d provider[s]", len(provs))
		resp.ProviderPeers = pb.PeerInfosToPBPeers(dht.network, provinfos)
	}

	// Find closest peer on given cluster to desired key and reply with that info
	closer := dht.betterPeersToQuery(pmes, CloserPeerCount)
	closerinfos := peer.PeerInfos(dht.peerstore, closer)
	if closer != nil {
		for _, pi := range closerinfos {
			log.Debugf("handleGetValue returning closer peer: '%s'", pi.ID)
			if len(pi.Addrs) < 1 {
				log.Critical("no addresses on peer being sent!")
			}
		}

		resp.CloserPeers = pb.PeerInfosToPBPeers(dht.network, closerinfos)
	}

	return resp, nil
}

// Store a value in this peer local storage
func (dht *IpfsDHT) handlePutValue(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	dskey := u.Key(pmes.GetKey()).DsKey()

	if err := dht.verifyRecord(pmes.GetRecord()); err != nil {
		fmt.Println(u.Key(pmes.GetRecord().GetAuthor()))
		log.Error("Bad dht record in put request")
		return nil, err
	}

	data, err := proto.Marshal(pmes.GetRecord())
	if err != nil {
		return nil, err
	}

	err = dht.datastore.Put(dskey, data)
	log.Debugf("%s handlePutValue %v\n", dht.self, dskey)
	return pmes, err
}

func (dht *IpfsDHT) handlePing(_ context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("%s Responding to ping from %s!\n", dht.self, p)
	return pmes, nil
}

func (dht *IpfsDHT) handleFindPeer(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	resp := pb.NewMessage(pmes.GetType(), "", pmes.GetClusterLevel())
	var closest []peer.ID

	// if looking for self... special case where we send it on CloserPeers.
	if peer.ID(pmes.GetKey()) == dht.self {
		closest = []peer.ID{dht.self}
	} else {
		closest = dht.betterPeersToQuery(pmes, CloserPeerCount)
	}

	if closest == nil {
		log.Errorf("handleFindPeer: could not find anything.")
		return resp, nil
	}

	var withAddresses []peer.PeerInfo
	closestinfos := peer.PeerInfos(dht.peerstore, closest)
	for _, pi := range closestinfos {
		if len(pi.Addrs) > 0 {
			withAddresses = append(withAddresses, pi)
			log.Debugf("handleFindPeer: sending back '%s'", pi.ID)
		}
	}

	resp.CloserPeers = pb.PeerInfosToPBPeers(dht.network, withAddresses)
	return resp, nil
}

func (dht *IpfsDHT) handleGetProviders(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	resp := pb.NewMessage(pmes.GetType(), pmes.GetKey(), pmes.GetClusterLevel())

	// check if we have this value, to add ourselves as provider.
	log.Debugf("handling GetProviders: '%s'", u.Key(pmes.GetKey()))
	dsk := u.Key(pmes.GetKey()).DsKey()
	has, err := dht.datastore.Has(dsk)
	if err != nil && err != ds.ErrNotFound {
		log.Errorf("unexpected datastore error: %v\n", err)
		has = false
	}

	// setup providers
	providers := dht.providers.GetProviders(ctx, u.Key(pmes.GetKey()))
	if has {
		providers = append(providers, dht.self)
	}

	if providers != nil && len(providers) > 0 {
		infos := peer.PeerInfos(dht.peerstore, providers)
		resp.ProviderPeers = pb.PeerInfosToPBPeers(dht.network, infos)
	}

	// Also send closer peers.
	closer := dht.betterPeersToQuery(pmes, CloserPeerCount)
	if closer != nil {
		infos := peer.PeerInfos(dht.peerstore, providers)
		resp.CloserPeers = pb.PeerInfosToPBPeers(dht.network, infos)
	}

	return resp, nil
}

type providerInfo struct {
	Creation time.Time
	Value    peer.ID
}

func (dht *IpfsDHT) handleAddProvider(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	key := u.Key(pmes.GetKey())

	log.Debugf("%s adding %s as a provider for '%s'\n", dht.self, p, peer.ID(key))

	// add provider should use the address given in the message
	pinfos := pb.PBPeersToPeerInfos(pmes.GetProviderPeers())
	for _, pi := range pinfos {
		if pi.ID != p {
			// we should ignore this provider reccord! not from originator.
			// (we chould sign them and check signature later...)
			log.Errorf("handleAddProvider received provider %s from %s. Ignore.", pi.ID, p)
			continue
		}

		if len(pi.Addrs) < 1 {
			log.Errorf("got no valid addresses for provider %s. Ignore.", p)
			continue
		}

		log.Infof("received provider %s for %s (addrs: %s)", p, key, pi.Addrs)
		for _, maddr := range pi.Addrs {
			// add the received addresses to our peerstore.
			dht.peerstore.AddAddress(p, maddr)
		}
		dht.providers.AddProvider(key, p)
	}

	return pmes, nil // send back same msg as confirmation.
}
