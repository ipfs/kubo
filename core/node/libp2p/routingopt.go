package libp2p

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/repo"
	irouting "github.com/ipfs/kubo/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dual "github.com/libp2p/go-libp2p-kad-dht/dual"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	routing "github.com/libp2p/go-libp2p/core/routing"
)

type RoutingOptionArgs struct {
	Ctx                           context.Context
	Host                          host.Host
	Repo                          repo.Repo
	Datastore                     datastore.Batching
	Validator                     record.Validator
	BootstrapPeers                []peer.AddrInfo
	OptimisticProvide             bool
	OptimisticProvideJobsPoolSize int
	LoopbackAddressesOnLanDHT     bool
}

type RoutingOption func(args RoutingOptionArgs) (routing.Routing, error)

var noopRouter = routinghelpers.Null{}

// EndpointCapabilities represents which routing operations are supported
type EndpointCapabilities struct {
	Providers bool
	Peers     bool
	IPNS      bool
}

// parseEndpointPath extracts the base URL and determines routing capabilities based on URL path
func parseEndpointPath(endpoint string) (baseURL string, capabilities EndpointCapabilities, err error) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", EndpointCapabilities{}, fmt.Errorf("invalid URL %q: %w", endpoint, err)
	}

	// Build base URL without path
	baseURL = fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	if parsedURL.Port() != "" {
		// Port is included in Host for URLs with non-standard ports
		baseURL = fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	}

	// Analyze path to determine capabilities
	path := strings.TrimPrefix(parsedURL.Path, "/")
	path = strings.TrimSuffix(path, "/")

	// Handle different path patterns
	switch {
	case path == "":
		// No path specified - assume all capabilities (backward compatibility)
		capabilities = EndpointCapabilities{
			Providers: true,
			Peers:     true,
			IPNS:      true,
		}
	case path == "routing/v1/providers":
		capabilities = EndpointCapabilities{
			Providers: true,
			Peers:     false,
			IPNS:      false,
		}
	case path == "routing/v1/peers":
		capabilities = EndpointCapabilities{
			Providers: false,
			Peers:     true,
			IPNS:      false,
		}
	case path == "routing/v1/ipns":
		capabilities = EndpointCapabilities{
			Providers: false,
			Peers:     false,
			IPNS:      true,
		}
	case strings.HasPrefix(path, "routing/v1/"):
		return "", EndpointCapabilities{}, fmt.Errorf("unsupported routing path %q in URL %q. Supported paths: /routing/v1/providers, /routing/v1/peers, /routing/v1/ipns", path, endpoint)
	default:
		return "", EndpointCapabilities{}, fmt.Errorf("invalid routing path %q in URL %q. Expected /routing/v1/* path or no path for full capabilities", path, endpoint)
	}

	return baseURL, capabilities, nil
}

func constructDefaultHTTPRouters(cfg *config.Config, r repo.Repo) ([]*routinghelpers.ParallelRouter, error) {
	var routers []*routinghelpers.ParallelRouter
	httpRetrievalEnabled := cfg.HTTPRetrieval.Enabled.WithDefault(config.DefaultHTTPRetrievalEnabled)

	// Use config.DefaultHTTPRouters if custom override was sent via config.EnvHTTPRouters
	// or if user did not set any preference in cfg.Routing.DelegatedRouters
	var httpRouterEndpoints []string
	if os.Getenv(config.EnvHTTPRouters) != "" || len(cfg.Routing.DelegatedRouters) == 0 {
		httpRouterEndpoints = config.DefaultHTTPRouters
	} else {
		httpRouterEndpoints = cfg.DelegatedRoutersWithAutoConfig(r.Path())
	}

	// Append HTTP routers for additional speed
	for _, endpoint := range httpRouterEndpoints {
		// Parse endpoint to determine capabilities and extract base URL
		baseURL, capabilities, err := parseEndpointPath(endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to parse routing endpoint %q: %w", endpoint, err)
		}

		// Construct HTTP router using base URL (without path)
		httpRouter, err := irouting.ConstructHTTPRouter(baseURL, cfg.Identity.PeerID, httpAddrsFromConfig(cfg.Addresses), cfg.Identity.PrivKey, httpRetrievalEnabled)
		if err != nil {
			return nil, err
		}

		// Configure router operations based on path-determined capabilities
		// https://specs.ipfs.tech/routing/http-routing-v1/
		r := &irouting.Composer{
			GetValueRouter:      noopRouter, // Default disabled
			PutValueRouter:      noopRouter, // IPNS publishing is handled by Ipns.DelegatedPublishers
			ProvideRouter:       noopRouter, // we don't have spec for sending provides to /routing/v1 (revisit once https://github.com/ipfs/specs/pull/378 or similar is ratified)
			FindPeersRouter:     noopRouter, // Default disabled
			FindProvidersRouter: noopRouter, // Default disabled
		}

		// Enable specific capabilities based on URL path
		if capabilities.IPNS {
			r.GetValueRouter = httpRouter // GET /routing/v1/ipns
		}
		if capabilities.Peers {
			r.FindPeersRouter = httpRouter // GET /routing/v1/peers
		}
		if capabilities.Providers {
			r.FindProvidersRouter = httpRouter // GET /routing/v1/providers
		}

		// Handle special cases and backward compatibility
		if baseURL == config.CidContactRoutingURL {
			// Special-case: cid.contact only supports /routing/v1/providers/cid
			// Ensure only providers capability is enabled for cid.contact
			r.GetValueRouter = noopRouter
			r.PutValueRouter = noopRouter
			r.ProvideRouter = noopRouter
			r.FindPeersRouter = noopRouter
			r.FindProvidersRouter = httpRouter // Only providers supported
		}

		routers = append(routers, &routinghelpers.ParallelRouter{
			Router:                  r,
			IgnoreError:             true,             // https://github.com/ipfs/kubo/pull/9475#discussion_r1042507387
			Timeout:                 15 * time.Second, // 5x server value from https://github.com/ipfs/kubo/pull/9475#discussion_r1042428529
			DoNotWaitForSearchValue: true,
			ExecuteAfter:            0,
		})
	}
	return routers, nil
}

func constructIPNSDelegatedPublishers(cfg *config.Config, r repo.Repo) ([]*routinghelpers.ParallelRouter, error) {
	var routers []*routinghelpers.ParallelRouter
	httpRetrievalEnabled := cfg.HTTPRetrieval.Enabled.WithDefault(config.DefaultHTTPRetrievalEnabled)

	// Use resolved IPNS delegated publishers
	ipnsPublishers := cfg.DelegatedPublishersWithAutoConfig(r.Path())

	// Construct HTTP routers specifically for IPNS publishing
	for _, endpoint := range ipnsPublishers {
		// Parse endpoint to determine capabilities and extract base URL
		baseURL, capabilities, err := parseEndpointPath(endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to parse IPNS publisher endpoint %q: %w", endpoint, err)
		}

		// Validate that this endpoint supports IPNS operations
		if !capabilities.IPNS && endpoint != baseURL {
			// If a path was specified but it doesn't support IPNS, that's an error
			return nil, fmt.Errorf("IPNS publisher endpoint %q does not support IPNS operations (path must be /routing/v1/ipns or no path)", endpoint)
		}

		// Construct HTTP router using base URL (without path)
		httpRouter, err := irouting.ConstructHTTPRouter(baseURL, cfg.Identity.PeerID, httpAddrsFromConfig(cfg.Addresses), cfg.Identity.PrivKey, httpRetrievalEnabled)
		if err != nil {
			return nil, err
		}

		// Create a composer that only enables PutValue for IPNS publishing
		r := &irouting.Composer{
			GetValueRouter:      noopRouter, // IPNS resolution is handled by Routing.DelegatedRouters
			PutValueRouter:      httpRouter, // PUT /routing/v1/ipns - This is what we want for publishing
			ProvideRouter:       noopRouter, // Not needed for IPNS publishing
			FindPeersRouter:     noopRouter, // Not needed for IPNS publishing
			FindProvidersRouter: noopRouter, // Not needed for IPNS publishing
		}

		routers = append(routers, &routinghelpers.ParallelRouter{
			Router:                  r,
			IgnoreError:             true,             // Continue to other publishers if one fails
			Timeout:                 15 * time.Second, // Same timeout as delegated routers
			DoNotWaitForSearchValue: true,
			ExecuteAfter:            0,
		})
	}
	return routers, nil
}

// ConstructDefaultRouting returns routers used when Routing.Type is unset or set to "auto"
func ConstructDefaultRouting(cfg *config.Config, routingOpt RoutingOption) RoutingOption {
	return func(args RoutingOptionArgs) (routing.Routing, error) {
		// Defined routers will be queried in parallel (optimizing for response speed)
		// Different trade-offs can be made by setting Routing.Type = "custom" with own Routing.Routers
		var routers []*routinghelpers.ParallelRouter

		dhtRouting, err := routingOpt(args)
		if err != nil {
			return nil, err
		}
		routers = append(routers, &routinghelpers.ParallelRouter{
			Router:                  dhtRouting,
			IgnoreError:             false,
			DoNotWaitForSearchValue: true,
			ExecuteAfter:            0,
		})

		httpRouters, err := constructDefaultHTTPRouters(cfg, args.Repo)
		if err != nil {
			return nil, err
		}

		routers = append(routers, httpRouters...)

		// Add IPNS delegated publishers
		ipnsPublishers, err := constructIPNSDelegatedPublishers(cfg, args.Repo)
		if err != nil {
			return nil, err
		}

		routers = append(routers, ipnsPublishers...)

		routing := routinghelpers.NewComposableParallel(routers)
		return routing, nil
	}
}

// constructDHTRouting is used when Routing.Type = "dht"
func constructDHTRouting(mode dht.ModeOpt) RoutingOption {
	return func(args RoutingOptionArgs) (routing.Routing, error) {
		dhtOpts := []dht.Option{
			dht.Concurrency(10),
			dht.Mode(mode),
			dht.Datastore(args.Datastore),
			dht.Validator(args.Validator),
		}
		if args.OptimisticProvide {
			dhtOpts = append(dhtOpts, dht.EnableOptimisticProvide())
		}
		if args.OptimisticProvideJobsPoolSize != 0 {
			dhtOpts = append(dhtOpts, dht.OptimisticProvideJobsPoolSize(args.OptimisticProvideJobsPoolSize))
		}
		wanOptions := []dht.Option{
			dht.BootstrapPeers(args.BootstrapPeers...),
		}
		lanOptions := []dht.Option{}
		if args.LoopbackAddressesOnLanDHT {
			lanOptions = append(lanOptions, dht.AddressFilter(nil))
		}
		return dual.New(
			args.Ctx, args.Host,
			dual.DHTOption(dhtOpts...),
			dual.WanDHTOption(wanOptions...),
			dual.LanDHTOption(lanOptions...),
		)
	}
}

// ConstructDelegatedRouting is used when Routing.Type = "custom"
func ConstructDelegatedRouting(routers config.Routers, methods config.Methods, peerID string, addrs config.Addresses, privKey string, httpRetrieval bool) RoutingOption {
	return func(args RoutingOptionArgs) (routing.Routing, error) {
		return irouting.Parse(routers, methods,
			&irouting.ExtraDHTParams{
				BootstrapPeers: args.BootstrapPeers,
				Host:           args.Host,
				Validator:      args.Validator,
				Datastore:      args.Datastore,
				Context:        args.Ctx,
			},
			&irouting.ExtraHTTPParams{
				PeerID:        peerID,
				Addrs:         httpAddrsFromConfig(addrs),
				PrivKeyB64:    privKey,
				HTTPRetrieval: httpRetrieval,
			},
		)
	}
}

func constructNilRouting(_ RoutingOptionArgs) (routing.Routing, error) {
	return routinghelpers.Null{}, nil
}

var (
	DHTOption       RoutingOption = constructDHTRouting(dht.ModeAuto)
	DHTClientOption               = constructDHTRouting(dht.ModeClient)
	DHTServerOption               = constructDHTRouting(dht.ModeServer)
	NilRouterOption               = constructNilRouting
)

// httpAddrsFromConfig creates a list of addresses from the provided configuration to be used by HTTP delegated routers.
func httpAddrsFromConfig(cfgAddrs config.Addresses) []string {
	// Swarm addrs are announced by default
	addrs := cfgAddrs.Swarm
	// if Announce addrs are specified - override Swarm
	if len(cfgAddrs.Announce) > 0 {
		addrs = cfgAddrs.Announce
	} else if len(cfgAddrs.NoAnnounce) > 0 {
		// if Announce adds are not specified - filter Swarm addrs with NoAnnounce list
		maddrs := map[string]struct{}{}
		for _, addr := range addrs {
			maddrs[addr] = struct{}{}
		}
		for _, addr := range cfgAddrs.NoAnnounce {
			delete(maddrs, addr)
		}
		addrs = make([]string, 0, len(maddrs))
		for k := range maddrs {
			addrs = append(addrs, k)
		}
	}
	// append AppendAnnounce addrs to the result list
	if len(cfgAddrs.AppendAnnounce) > 0 {
		addrs = append(addrs, cfgAddrs.AppendAnnounce...)
	}
	return addrs
}
