package routing

import (
	"strconv"

	drc "github.com/ipfs/go-delegated-routing/client"
	drp "github.com/ipfs/go-delegated-routing/gen/proto"
	"github.com/ipfs/go-ipfs/config"
	"github.com/libp2p/go-libp2p-core/routing"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
)

type TieredRouter interface {
	routing.Routing
	ProviderManyWrapper() ProvideMany
}

var _ TieredRouter = &Tiered{}

// Tiered is a routing Tiered implementation providing some extra methods to fill
// some special use cases when initializing the client.
type Tiered struct {
	routinghelpers.Tiered
}

// ProviderManyWrapper returns a ProviderMany implementation including all Routers that
// implements ProviderMany
func (ds Tiered) ProviderManyWrapper() ProvideMany {
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
func RoutingFromConfig(c config.Router) (routing.Routing, error) {
	switch {
	case c.Type == string(config.RouterTypeReframe):
		return reframeRoutingFromConfig(c)
	case c.Type == string(config.RouterTypeDHT):
		return dhtRoutingFromConfig(c)
	default:
		return nil, &RouterTypeNotFoundError{c.Type}
	}
}

func dhtRoutingFromConfig(conf config.Router) (routing.Routing, error) {
	panic("implement this")
}

func reframeRoutingFromConfig(conf config.Router) (routing.Routing, error) {
	var dr drp.DelegatedRouting_Client

	param := string(config.RouterParamAddress)
	addr, ok := conf.Parameters[param]
	if !ok {
		return nil, NewParamNeededErr(param, conf.Type)
	}

	dr, err := drp.New_DelegatedRouting_Client(addr)
	if err != nil {
		return nil, err
	}

	c := drc.NewClient(dr)
	crc := drc.NewContentRoutingClient(c)
	return &reframeRoutingWrapper{
		Client:               c,
		ContentRoutingClient: crc,
	}, nil
}
