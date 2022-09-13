package routing

import (
	"encoding/base64"
	"strconv"

	drc "github.com/ipfs/go-delegated-routing/client"
	drp "github.com/ipfs/go-delegated-routing/gen/proto"
	"github.com/ipfs/kubo/config"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multicodec"
)

type TieredRouter interface {
	routing.Routing
	ProvideMany() ProvideMany
}

var _ TieredRouter = &Tiered{}

// Tiered is a routing Tiered implementation providing some extra methods to fill
// some special use cases when initializing the client.
type Tiered struct {
	routinghelpers.Tiered
}

// ProvideMany returns a ProvideMany implementation including all Routers that
// implements ProvideMany
func (ds Tiered) ProvideMany() ProvideMany {
	var pms []ProvideMany
	for _, r := range ds.Tiered.Routers {
		pm, ok := r.(ProvideMany)
		if !ok {
			continue
		}
		pms = append(pms, pm)
	}

	if len(pms) == 0 {
		return nil
	}

	return &ProvideManyWrapper{pms: pms}
}

const defaultPriority = 100000

// GetPriority extract priority from config params.
// Small numbers represent more important routers.
func GetPriority(params map[string]string) int {
	param := params[string(config.RouterParamPriority)]
	if param == "" {
		return defaultPriority
	}

	p, err := strconv.Atoi(param)
	if err != nil {
		return defaultPriority
	}

	return p
}

// RoutingFromConfig creates a Routing instance from the specified configuration.
// peerID, addrs, priv are optional, however Provide and ProvideMany APIs will not work
// if these parameters aren't provided.
func RoutingFromConfig(c config.Router, peerID string, addrs []string, priv string) (routing.Routing, error) {
	switch {
	case c.Type == string(config.RouterTypeReframe):
		return reframeRoutingFromConfig(c, peerID, addrs, priv)
	default:
		return nil, &RouterTypeNotFoundError{c.Type}
	}
}

func reframeRoutingFromConfig(conf config.Router, peerID string, addrs []string, priv string) (routing.Routing, error) {
	var dr drp.DelegatedRouting_Client

	param := string(config.RouterParamEndpoint)
	addr, ok := conf.Parameters[param]
	if !ok {
		return nil, NewParamNeededErr(param, conf.Type)
	}

	dr, err := drp.New_DelegatedRouting_Client(addr)
	if err != nil {
		return nil, err
	}

	var provider *drc.Provider
	if len(peerID) != 0 {
		pID, err := peer.Decode(peerID)
		if err != nil {
			return nil, err
		}

		multiaddrs := make([]ma.Multiaddr, len(addrs))
		for i, a := range addrs {
			multiaddrs[i], err = ma.NewMultiaddr(a)
			if err != nil {
				return nil, err
			}
		}

		provider = &drc.Provider{
			Peer: peer.AddrInfo{
				ID:    pID,
				Addrs: multiaddrs},
			ProviderProto: []drc.TransferProtocol{{Codec: multicodec.TransportBitswap}}}

	}

	var key ic.PrivKey
	if len(priv) != 0 {
		pkb, err := base64.StdEncoding.DecodeString(priv)
		if err != nil {
			return nil, err
		}

		key, err = ic.UnmarshalPrivateKey([]byte(pkb))
		if err != nil {
			return nil, err
		}
	}

	c, err := drc.NewClient(dr, provider, key)
	if err != nil {
		return nil, err
	}
	crc := drc.NewContentRoutingClient(c)
	return &reframeRoutingWrapper{
		Client:               c,
		ContentRoutingClient: crc,
	}, nil
}
