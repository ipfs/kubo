package grandcentral

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	dhtpb "github.com/jbenet/go-ipfs/routing/dht/pb"
	proxy "github.com/jbenet/go-ipfs/routing/grandcentral/proxy"
	util "github.com/jbenet/go-ipfs/util"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

// Server handles routing queries using a database backend
type Server struct {
	local           peer.ID
	datastore       datastore.ThreadSafeDatastore
	dialer          inet.Network
	peerstore       peer.Peerstore
	*proxy.Loopback // so server can be injected into client
}

// NewServer creates a new GrandCentral routing Server
func NewServer(ds datastore.ThreadSafeDatastore, d inet.Network, ps peer.Peerstore, local peer.ID) (*Server, error) {
	s := &Server{local, ds, d, ps, nil}
	s.Loopback = &proxy.Loopback{
		Handler: s,
		Local:   local,
	}
	return s, nil
}

// HandleLocalRequest implements the proxy.RequestHandler interface. This is
// where requests are received from the outside world.
func (s *Server) HandleRequest(ctx context.Context, p peer.ID, req *dhtpb.Message) *dhtpb.Message {
	_, response := s.handleMessage(ctx, p, req) // ignore response peer. it's local.
	return response
}

// TODO extract backend. backend can be implemented with whatever database we desire
func (s *Server) handleMessage(
	ctx context.Context, p peer.ID, req *dhtpb.Message) (peer.ID, *dhtpb.Message) {

	//  FIXME threw everything into this switch statement to get things going.
	//  Once each operation is well-defined, extract pluggable backend so any
	//  database may be used.

	var response = dhtpb.NewMessage(req.GetType(), req.GetKey(), req.GetClusterLevel())
	switch req.GetType() {

	case dhtpb.Message_GET_VALUE:
		dskey := util.Key(req.GetKey()).DsKey()
		val, err := s.datastore.Get(dskey)
		if err != nil {
			log.Error(errors.Wrap(err))
			return "", nil
		}
		rawRecord, ok := val.([]byte)
		if !ok {
			log.Errorf("datastore had non byte-slice value for %v", dskey)
			return "", nil
		}
		if err := proto.Unmarshal(rawRecord, response.Record); err != nil {
			log.Error("failed to unmarshal dht record from datastore")
			return "", nil
		}
		// TODO before merging: if we know any providers for the requested value, return those.
		return p, response

	case dhtpb.Message_PUT_VALUE:
		// TODO before merging: verifyRecord(req.GetRecord())
		data, err := proto.Marshal(req.GetRecord())
		if err != nil {
			log.Error(err)
			return "", nil
		}
		dskey := util.Key(req.GetKey()).DsKey()
		if err := s.datastore.Put(dskey, data); err != nil {
			log.Error(err)
			return "", nil
		}
		return p, req // TODO before merging: verify that we should return record

	case dhtpb.Message_FIND_NODE:
		p := s.peerstore.PeerInfo(peer.ID(req.GetKey()))
		response.CloserPeers = dhtpb.PeerInfosToPBPeers(s.dialer, []peer.PeerInfo{p})
		return p.ID, response

	case dhtpb.Message_ADD_PROVIDER:
		for _, provider := range req.GetProviderPeers() {
			providerID := peer.ID(provider.GetId())
			if providerID != p {
				log.Errorf("provider message came from third-party %s", p)
				continue
			}
			for _, maddr := range provider.Addresses() {
				// FIXME do we actually want to store to peerstore
				s.peerstore.AddAddr(p, maddr, peer.TempAddrTTL)
			}
		}
		var providers []dhtpb.Message_Peer
		pkey := datastore.KeyWithNamespaces([]string{"routing", "providers", req.GetKey()})
		if v, err := s.datastore.Get(pkey); err == nil {
			if protopeers, ok := v.([]dhtpb.Message_Peer); ok {
				providers = append(providers, protopeers...)
			}
		}
		if err := s.datastore.Put(pkey, providers); err != nil {
			log.Error(err)
			return "", nil
		}
		return "", nil

	case dhtpb.Message_GET_PROVIDERS:
		dskey := util.Key(req.GetKey()).DsKey()
		exists, err := s.datastore.Has(dskey)
		if err == nil && exists {
			response.ProviderPeers = append(response.ProviderPeers, dhtpb.PeerInfosToPBPeers(s.dialer, []peer.PeerInfo{peer.PeerInfo{ID: s.local}})...)
		}
		// FIXME(btc) is this how we want to persist this data?
		pkey := datastore.KeyWithNamespaces([]string{"routing", "providers", req.GetKey()})
		if v, err := s.datastore.Get(pkey); err == nil {
			if protopeers, ok := v.([]dhtpb.Message_Peer); ok {
				for _, p := range protopeers {
					response.ProviderPeers = append(response.ProviderPeers, &p)
				}
			}
		}
		return p, response

	case dhtpb.Message_PING:
		return p, req
	default:
	}
	return "", nil
}

var _ proxy.RequestHandler = &Server{}
var _ proxy.Proxy = &Server{}
