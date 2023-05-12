package routing

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	drclient "github.com/ipfs/boxo/routing/http/client"
	"github.com/ipfs/boxo/routing/http/contentrouter"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/config"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"go.opencensus.io/stats/view"
)

var log = logging.Logger("routing/delegated")

func Parse(routers config.Routers, methods config.Methods, extraDHT *ExtraDHTParams) (routing.Routing, error) {
	if err := methods.Check(); err != nil {
		return nil, err
	}

	createdRouters := make(map[string]routing.Routing)
	finalRouter := &Composer{}

	// Create all needed routers from method names
	for mn, m := range methods {
		router, err := parse(make(map[string]bool), createdRouters, m.RouterName, routers, extraDHT)
		if err != nil {
			return nil, err
		}

		switch mn {
		case config.MethodNamePutIPNS:
			finalRouter.PutValueRouter = router
		case config.MethodNameGetIPNS:
			finalRouter.GetValueRouter = router
		case config.MethodNameFindPeers:
			finalRouter.FindPeersRouter = router
		case config.MethodNameFindProviders:
			finalRouter.FindProvidersRouter = router
		case config.MethodNameProvide:
			finalRouter.ProvideRouter = router
		}

		log.Info("using method ", mn, " with router ", m.RouterName)
	}

	return finalRouter, nil
}

func parse(visited map[string]bool,
	createdRouters map[string]routing.Routing,
	routerName string,
	routersCfg config.Routers,
	extraDHT *ExtraDHTParams,
) (routing.Routing, error) {
	// check if we already created it
	r, ok := createdRouters[routerName]
	if ok {
		return r, nil
	}

	// check if we are in a dep loop
	if visited[routerName] {
		return nil, fmt.Errorf("dependency loop creating router with name %q", routerName)
	}

	// set node as visited
	visited[routerName] = true

	cfg, ok := routersCfg[routerName]
	if !ok {
		return nil, fmt.Errorf("config for router with name %q not found", routerName)
	}

	var router routing.Routing
	var err error
	switch cfg.Type {
	case config.RouterTypeHTTP:
		router, err = httpRoutingFromConfig(cfg.Router)
	case config.RouterTypeDHT:
		router, err = dhtRoutingFromConfig(cfg.Router, extraDHT)
	case config.RouterTypeParallel:
		crp := cfg.Parameters.(*config.ComposableRouterParams)
		var pr []*routinghelpers.ParallelRouter
		for _, cr := range crp.Routers {
			ri, err := parse(visited, createdRouters, cr.RouterName, routersCfg, extraDHT)
			if err != nil {
				return nil, err
			}

			pr = append(pr, &routinghelpers.ParallelRouter{
				Router:                  ri,
				IgnoreError:             cr.IgnoreErrors,
				DoNotWaitForSearchValue: true,
				Timeout:                 cr.Timeout.Duration,
				ExecuteAfter:            cr.ExecuteAfter.WithDefault(0),
			})

		}

		router = routinghelpers.NewComposableParallel(pr)
	case config.RouterTypeSequential:
		crp := cfg.Parameters.(*config.ComposableRouterParams)
		var sr []*routinghelpers.SequentialRouter
		for _, cr := range crp.Routers {
			ri, err := parse(visited, createdRouters, cr.RouterName, routersCfg, extraDHT)
			if err != nil {
				return nil, err
			}

			sr = append(sr, &routinghelpers.SequentialRouter{
				Router:      ri,
				IgnoreError: cr.IgnoreErrors,
				Timeout:     cr.Timeout.Duration,
			})

		}

		router = routinghelpers.NewComposableSequential(sr)
	default:
		return nil, fmt.Errorf("unknown router type %q", cfg.Type)
	}

	if err != nil {
		return nil, err
	}

	createdRouters[routerName] = router

	log.Info("created router ", routerName, " with params ", cfg.Parameters)

	return router, nil
}

func ConstructHTTPRouter(endpoint string, peerID string, addrs []string, privKey string) (routing.Routing, error) {
	return httpRoutingFromConfig(
		config.Router{
			Type: "http",
			Parameters: &config.HTTPRouterParams{
				Endpoint: endpoint,
			},
		},
	)
}

func httpRoutingFromConfig(conf config.Router) (routing.Routing, error) {
	params := conf.Parameters.(*config.HTTPRouterParams)
	if params.Endpoint == "" {
		return nil, NewParamNeededErr("Endpoint", conf.Type)
	}

	// Increase per-host connection pool since we are making lots of concurrent requests.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 500
	transport.MaxIdleConnsPerHost = 100

	delegateHTTPClient := &http.Client{
		Transport: &drclient.ResponseBodyLimitedTransport{
			RoundTripper: transport,
			LimitBytes:   1 << 20,
		},
	}

	cli, err := drclient.New(
		params.Endpoint,
		drclient.WithHTTPClient(delegateHTTPClient),
		drclient.WithUserAgent(version.GetUserAgentVersion()),
	)
	if err != nil {
		return nil, err
	}

	cr := contentrouter.NewContentRoutingClient(
		cli,
	)

	err = view.Register(drclient.OpenCensusViews...)
	if err != nil {
		return nil, fmt.Errorf("registering HTTP delegated routing views: %w", err)
	}

	return &httpRoutingWrapper{
		ContentRouting:    cr,
		PeerRouting:       cr,
		ValueStore:        cr,
		ProvideManyRouter: cr,
	}, nil
}

type ExtraDHTParams struct {
	BootstrapPeers []peer.AddrInfo
	Host           host.Host
	Validator      record.Validator
	Datastore      datastore.Batching
	Context        context.Context
}

func dhtRoutingFromConfig(conf config.Router, extra *ExtraDHTParams) (routing.Routing, error) {
	params, ok := conf.Parameters.(*config.DHTRouterParams)
	if !ok {
		return nil, errors.New("incorrect params for DHT router")
	}

	if params.AcceleratedDHTClient {
		return createFullRT(extra)
	}

	var mode dht.ModeOpt
	switch params.Mode {
	case config.DHTModeAuto:
		mode = dht.ModeAuto
	case config.DHTModeClient:
		mode = dht.ModeClient
	case config.DHTModeServer:
		mode = dht.ModeServer
	default:
		return nil, fmt.Errorf("invalid DHT mode: %q", params.Mode)
	}

	return createDHT(extra, params.PublicIPNetwork, mode)
}

func createDHT(params *ExtraDHTParams, public bool, mode dht.ModeOpt) (routing.Routing, error) {
	var opts []dht.Option

	if public {
		opts = append(opts, dht.QueryFilter(dht.PublicQueryFilter),
			dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
			dht.RoutingTablePeerDiversityFilter(dht.NewRTPeerDiversityFilter(params.Host, 2, 3)))
	} else {
		opts = append(opts, dht.ProtocolExtension(dual.LanExtension),
			dht.QueryFilter(dht.PrivateQueryFilter),
			dht.RoutingTableFilter(dht.PrivateRoutingTableFilter))
	}

	opts = append(opts,
		dht.Concurrency(10),
		dht.Mode(mode),
		dht.Datastore(params.Datastore),
		dht.Validator(params.Validator),
		dht.BootstrapPeers(params.BootstrapPeers...))

	return dht.New(
		params.Context, params.Host, opts...,
	)
}

func createFullRT(params *ExtraDHTParams) (routing.Routing, error) {
	return fullrt.NewFullRT(params.Host,
		dht.DefaultPrefix,
		fullrt.DHTOption(
			dht.Validator(params.Validator),
			dht.Datastore(params.Datastore),
			dht.BootstrapPeers(params.BootstrapPeers...),
			dht.BucketSize(20),
		),
	)
}
