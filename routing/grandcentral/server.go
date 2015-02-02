package grandcentral

import (
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	dhtpb "github.com/jbenet/go-ipfs/routing/dht/pb"
	proxy "github.com/jbenet/go-ipfs/routing/grandcentral/proxy"
	util "github.com/jbenet/go-ipfs/util"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

// Server handles routing queries using a database backend
type Server struct {
	local           peer.ID
	routingBackend  datastore.ThreadSafeDatastore
	peerstore       peer.Peerstore
	*proxy.Loopback // so server can be injected into client
}

// NewServer creates a new GrandCentral routing Server
func NewServer(ds datastore.ThreadSafeDatastore, ps peer.Peerstore, local peer.ID) (*Server, error) {
	s := &Server{local, ds, ps, nil}
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

	log.EventBegin(ctx, "routingMessageReceived", req, p, s.local).Done() // TODO may need to differentiate between local and remote

	var response = dhtpb.NewMessage(req.GetType(), req.GetKey(), req.GetClusterLevel())
	switch req.GetType() {

	case dhtpb.Message_GET_VALUE:
		rawRecord, err := getRoutingRecord(s.routingBackend, util.Key(req.GetKey()))
		if err != nil {
			return "", nil
		}
		response.Record = rawRecord
		// TODO before merging: if we know any providers for the requested value, return those.
		return p, response

	case dhtpb.Message_PUT_VALUE:
		// TODO before merging: verifyRecord(req.GetRecord())
		putRoutingRecord(s.routingBackend, util.Key(req.GetKey()), req.GetRecord())
		return p, req // TODO before merging: verify that we should return record

	case dhtpb.Message_FIND_NODE:
		p := s.peerstore.PeerInfo(peer.ID(req.GetKey()))
		pri := []dhtpb.PeerRoutingInfo{
			dhtpb.PeerRoutingInfo{
				PeerInfo: p,
				// Connectedness: TODO
			},
		}
		response.CloserPeers = dhtpb.PeerRoutingInfosToPBPeers(pri)
		return p.ID, response

	case dhtpb.Message_ADD_PROVIDER:
		// FIXME(btc): do we want to store these locally? I think the
		// storeProvidersToPeerstore behavior is straight from the DHT message
		// handler.
		storeProvidersToPeerstore(s.peerstore, p, req.GetProviderPeers())

		if err := putRoutingProviders(s.routingBackend, util.Key(req.GetKey()), req.GetProviderPeers()); err != nil {
			return "", nil
		}
		return "", nil

	case dhtpb.Message_GET_PROVIDERS:
		providers, err := getRoutingProviders(s.routingBackend, util.Key(req.GetKey()))
		if err != nil {
			return "", nil
		}
		response.ProviderPeers = providers
		return p, response

	case dhtpb.Message_PING:
		return p, req
	default:
	}
	return "", nil
}

var _ proxy.RequestHandler = &Server{}
var _ proxy.Proxy = &Server{}

func getRoutingRecord(ds datastore.Datastore, k util.Key) (*dhtpb.Record, error) {
	dskey := k.DsKey()
	val, err := ds.Get(dskey)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	recordBytes, ok := val.([]byte)
	if !ok {
		return nil, fmt.Errorf("datastore had non byte-slice value for %v", dskey)
	}
	var record dhtpb.Record
	if err := proto.Unmarshal(recordBytes, &record); err != nil {
		return nil, errors.New("failed to unmarshal dht record from datastore")
	}
	return &record, nil
}

func putRoutingRecord(ds datastore.Datastore, k util.Key, value *dhtpb.Record) error {
	data, err := proto.Marshal(value)
	if err != nil {
		return err
	}
	dskey := k.DsKey()
	// TODO namespace
	if err := ds.Put(dskey, data); err != nil {
		return err
	}
	return nil
}

func putRoutingProviders(ds datastore.Datastore, k util.Key, providers []*dhtpb.Message_Peer) error {
	log.Event(context.Background(), "putRoutingProviders", &k)
	pkey := datastore.KeyWithNamespaces([]string{"routing", "providers", k.String()})
	if v, err := ds.Get(pkey); err == nil {
		if msg, ok := v.([]byte); ok {
			var protomsg dhtpb.Message
			if err := proto.Unmarshal(msg, &protomsg); err != nil {
				log.Error("failed to unmarshal routing provider record. programmer error")
			} else {
				providers = append(providers, protomsg.ProviderPeers...)
			}
		}
	}
	var protomsg dhtpb.Message
	protomsg.ProviderPeers = providers
	data, err := proto.Marshal(&protomsg)
	if err != nil {
		return err
	}
	return ds.Put(pkey, data)
}

func storeProvidersToPeerstore(ps peer.Peerstore, p peer.ID, providers []*dhtpb.Message_Peer) {
	for _, provider := range providers {
		providerID := peer.ID(provider.GetId())
		if providerID != p {
			log.Errorf("provider message came from third-party %s", p)
			continue
		}
		for _, maddr := range provider.Addresses() {
			// as a router, we want to store addresses for peers who have provided
			ps.AddAddr(p, maddr, peer.AddressTTL)
		}
	}
}

func getRoutingProviders(ds datastore.Datastore, k util.Key) ([]*dhtpb.Message_Peer, error) {
	e := log.EventBegin(context.Background(), "getProviders", &k)
	defer e.Done()
	var providers []*dhtpb.Message_Peer
	pkey := datastore.KeyWithNamespaces([]string{"routing", "providers", k.String()}) // TODO key fmt
	if v, err := ds.Get(pkey); err == nil {
		if data, ok := v.([]byte); ok {
			var msg dhtpb.Message
			if err := proto.Unmarshal(data, &msg); err != nil {
				return nil, err
			}
			providers = append(providers, msg.GetProviderPeers()...)
		}
	}
	return providers, nil
}
