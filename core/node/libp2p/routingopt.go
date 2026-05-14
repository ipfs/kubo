package libp2p

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ipfs/boxo/autoconf"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/kubo/config"
	irouting "github.com/ipfs/kubo/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dual "github.com/libp2p/go-libp2p-kad-dht/dual"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	routing "github.com/libp2p/go-libp2p/core/routing"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
)

type RoutingOptionArgs struct {
	Ctx                           context.Context
	Host                          host.Host
	Datastore                     datastore.Batching
	Validator                     record.Validator
	BootstrapPeers                []peer.AddrInfo
	OptimisticProvide             bool
	OptimisticProvideJobsPoolSize int
	LoopbackAddressesOnLanDHT     bool
}

type RoutingOption func(args RoutingOptionArgs) (routing.Routing, error)

var noopRouter = routinghelpers.Null{}

// EndpointSource tracks where a URL came from to determine appropriate capabilities
type EndpointSource struct {
	URL           string
	SupportsRead  bool // came from DelegatedRoutersWithAutoConf (Read operations)
	SupportsWrite bool // came from DelegatedPublishersWithAutoConf (Write operations)
}

// determineCapabilities determines endpoint capabilities based on URL path and source
func determineCapabilities(endpoint EndpointSource) (string, autoconf.EndpointCapabilities, error) {
	parsed, err := autoconf.DetermineKnownCapabilities(endpoint.URL, endpoint.SupportsRead, endpoint.SupportsWrite)
	if err != nil {
		log.Debugf("Skipping endpoint %q: %v", endpoint.URL, err)
		return "", autoconf.EndpointCapabilities{}, nil // Return empty caps, not error
	}

	return parsed.BaseURL, parsed.Capabilities, nil
}

// collectAllEndpoints gathers URLs from both router and publisher sources
func collectAllEndpoints(cfg *config.Config) []EndpointSource {
	var endpoints []EndpointSource

	// Get router URLs (Read operations)
	var routerURLs []string
	if envRouters := os.Getenv(config.EnvHTTPRouters); envRouters != "" {
		// Use environment variable override if set (space or comma separated)
		splitFunc := func(r rune) bool { return r == ',' || r == ' ' }
		routerURLs = strings.FieldsFunc(envRouters, splitFunc)
		log.Warnf("Using HTTP routers from %s environment variable instead of config/autoconf: %v", config.EnvHTTPRouters, routerURLs)
	} else {
		// Use delegated routers from autoconf
		routerURLs = cfg.DelegatedRoutersWithAutoConf()
		// No fallback - if autoconf doesn't provide endpoints, use empty list
		// This exposes any autoconf issues rather than masking them with hardcoded defaults
	}

	// Add router URLs to collection
	for _, url := range routerURLs {
		endpoints = append(endpoints, EndpointSource{
			URL:           url,
			SupportsRead:  true,
			SupportsWrite: false,
		})
	}

	// Get publisher URLs (Write operations)
	publisherURLs := cfg.DelegatedPublishersWithAutoConf()

	// Add publisher URLs, merging with existing router URLs if they match
	for _, url := range publisherURLs {
		found := false
		for i, existing := range endpoints {
			if existing.URL == url {
				endpoints[i].SupportsWrite = true
				found = true
				break
			}
		}
		if !found {
			endpoints = append(endpoints, EndpointSource{
				URL:           url,
				SupportsRead:  false,
				SupportsWrite: true,
			})
		}
	}

	return endpoints
}

func constructDefaultHTTPRouters(cfg *config.Config, addrFunc func() []ma.Multiaddr) ([]*routinghelpers.ParallelRouter, error) {
	var routers []*routinghelpers.ParallelRouter
	httpRetrievalEnabled := cfg.HTTPRetrieval.Enabled.WithDefault(config.DefaultHTTPRetrievalEnabled)

	// Collect URLs from both router and publisher sources
	endpoints := collectAllEndpoints(cfg)

	// Group endpoints by origin (base URL) and aggregate capabilities
	originCapabilities := make(map[string]autoconf.EndpointCapabilities)
	for _, endpoint := range endpoints {
		// Parse endpoint and determine capabilities based on source
		baseURL, capabilities, err := determineCapabilities(endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to parse endpoint %q: %w", endpoint.URL, err)
		}

		// Aggregate capabilities for this origin
		existing := originCapabilities[baseURL]
		existing.Merge(capabilities)
		originCapabilities[baseURL] = existing
	}

	// Create single HTTP router and composer per origin
	for baseURL, capabilities := range originCapabilities {
		// Construct HTTP router using base URL (without path)
		httpRouter, err := irouting.ConstructHTTPRouter(baseURL, cfg.Identity.PeerID, addrFunc, cfg.Identity.PrivKey, httpRetrievalEnabled)
		if err != nil {
			return nil, err
		}

		// Configure router operations based on aggregated capabilities
		// https://specs.ipfs.tech/routing/http-routing-v1/
		composer := &irouting.Composer{
			GetValueRouter:      noopRouter, // Default disabled, enabled below based on capabilities
			PutValueRouter:      noopRouter, // Default disabled, enabled below based on capabilities
			ProvideRouter:       noopRouter, // we don't have spec for sending provides to /routing/v1 (revisit once https://github.com/ipfs/specs/pull/378 or similar is ratified)
			FindPeersRouter:     noopRouter, // Default disabled, enabled below based on capabilities
			FindProvidersRouter: noopRouter, // Default disabled, enabled below based on capabilities
		}

		// Enable specific capabilities
		if capabilities.IPNSGet {
			composer.GetValueRouter = httpRouter // GET /routing/v1/ipns for IPNS resolution
		}
		if capabilities.IPNSPut {
			composer.PutValueRouter = httpRouter // PUT /routing/v1/ipns for IPNS publishing
		}
		if capabilities.Peers {
			composer.FindPeersRouter = httpRouter // GET /routing/v1/peers
		}
		if capabilities.Providers {
			composer.FindProvidersRouter = httpRouter // GET /routing/v1/providers
		}

		// Handle special cases and backward compatibility
		if baseURL == config.CidContactRoutingURL {
			// Special-case: cid.contact only supports /routing/v1/providers/cid endpoint
			// Override any capabilities detected from URL path to ensure only providers is enabled
			// TODO: Consider moving this to configuration or removing once cid.contact adds more capabilities
			composer.GetValueRouter = noopRouter
			composer.PutValueRouter = noopRouter
			composer.ProvideRouter = noopRouter
			composer.FindPeersRouter = noopRouter
			composer.FindProvidersRouter = httpRouter // Only providers supported
		}

		routers = append(routers, &routinghelpers.ParallelRouter{
			Router:                  composer,
			IgnoreError:             true,             // https://github.com/ipfs/kubo/pull/9475#discussion_r1042507387
			Timeout:                 15 * time.Second, // 5x server value from https://github.com/ipfs/kubo/pull/9475#discussion_r1042428529
			DoNotWaitForSearchValue: true,
			ExecuteAfter:            0,
		})
	}
	return routers, nil
}

// ConstructDelegatedOnlyRouting returns routers used when Routing.Type is set to "delegated"
// This provides HTTP-only routing without DHT, using only delegated routers and IPNS publishers.
// Useful for environments where DHT connectivity is not available or desired
func ConstructDelegatedOnlyRouting(cfg *config.Config) RoutingOption {
	return func(args RoutingOptionArgs) (routing.Routing, error) {
		// Use only HTTP routers (includes both read and write capabilities) - no DHT
		var routers []*routinghelpers.ParallelRouter

		// Add HTTP delegated routers (includes both router and publisher capabilities)
		addrFunc := httpRouterAddrFunc(args.Host, cfg.Addresses)
		httpRouters, err := constructDefaultHTTPRouters(cfg, addrFunc)
		if err != nil {
			return nil, err
		}
		routers = append(routers, httpRouters...)

		// Validate that we have at least one router configured
		if len(routers) == 0 {
			return nil, fmt.Errorf("no delegated routers or publishers configured for 'delegated' routing mode")
		}

		routing := routinghelpers.NewComposableParallel(routers)
		return routing, nil
	}
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

		addrFunc := httpRouterAddrFunc(args.Host, cfg.Addresses)
		httpRouters, err := constructDefaultHTTPRouters(cfg, addrFunc)
		if err != nil {
			return nil, err
		}

		routers = append(routers, httpRouters...)

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
		// In stub mode, allow loopback peers in the WAN routing
		// table so Provide/PutValue work with ephemeral test peers.
		if os.Getenv("TEST_DHT_STUB") != "" {
			wanOptions = append(wanOptions,
				dht.AddressFilter(nil),
				dht.QueryFilter(func(_ any, _ peer.AddrInfo) bool { return true }),
				dht.RoutingTableFilter(func(_ any, _ peer.ID) bool { return true }),
				dht.RoutingTablePeerDiversityFilter(nil),
			)
		}
		lanOptions := []dht.Option{}
		if args.LoopbackAddressesOnLanDHT {
			lanOptions = append(lanOptions, dht.AddressFilter(nil))
		}
		d, err := dual.New(
			args.Ctx, args.Host,
			dual.DHTOption(dhtOpts...),
			dual.WanDHTOption(wanOptions...),
			dual.LanDHTOption(lanOptions...),
		)
		if err != nil {
			return nil, err
		}
		return d, nil
	}
}

// ConstructDelegatedRouting is used when Routing.Type = "custom"
func ConstructDelegatedRouting(routers config.Routers, methods config.Methods, peerID string, addrs config.Addresses, privKey string, httpRetrieval bool) RoutingOption {
	return func(args RoutingOptionArgs) (routing.Routing, error) {
		addrFunc := httpRouterAddrFunc(args.Host, addrs)
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
				AddrFunc:      addrFunc,
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

// confirmedAddrsHost matches libp2p hosts that support AutoNAT V2 address confirmation.
type confirmedAddrsHost interface {
	ConfirmedAddrs() (reachable, unreachable, unknown []ma.Multiaddr)
}

// Compile-time check: BasicHost must satisfy confirmedAddrsHost.
// ConfirmedAddrs is not part of the core host.Host interface and is marked
// experimental in go-libp2p. If BasicHost ever drops or changes this method,
// this assertion will fail at build time. In that case, update
// httpRouterAddrFunc (this file) and the swarm autonat command
// (core/commands/swarm_addrs_autonat.go) which both type-assert to this
// interface.
var _ confirmedAddrsHost = (*basichost.BasicHost)(nil)

// httpRouterAddrFunc returns a function that resolves provider addresses for
// HTTP routers at provide-time.
//
// Resolution logic:
//   - If Announce is set, use it as a static override (no dynamic resolution).
//   - Otherwise, prefer AutoNAT V2 confirmed reachable addresses when available,
//     falling back to host.Addrs() which resolves 0.0.0.0/:: Swarm binds to
//     concrete interface addresses and applies the libp2p AddrsFactory
//     (Addresses.NoAnnounce CIDR filters and Swarm.AddrFilters).
//   - AppendAnnounce addresses are always appended.
func httpRouterAddrFunc(h host.Host, cfgAddrs config.Addresses) func() []ma.Multiaddr {
	appendAddrs := parseMultiaddrs(cfgAddrs.AppendAnnounce)

	// If Announce is explicitly set, use it as a static override.
	if len(cfgAddrs.Announce) > 0 {
		staticAddrs := slices.Concat(parseMultiaddrs(cfgAddrs.Announce), appendAddrs)
		return func() []ma.Multiaddr { return staticAddrs }
	}

	ch, hasConfirmed := h.(confirmedAddrsHost)
	return func() []ma.Multiaddr {
		if hasConfirmed {
			reachable, _, _ := ch.ConfirmedAddrs()
			if len(reachable) > 0 {
				if len(appendAddrs) == 0 {
					return reachable
				}
				return slices.Concat(reachable, appendAddrs)
			}
		}
		// Fallback: host.Addrs() resolves wildcard binds (0.0.0.0, ::) to
		// concrete interface addresses and applies the libp2p AddrsFactory,
		// which is where Addresses.NoAnnounce CIDR filtering happens.
		hostAddrs := h.Addrs()
		if len(appendAddrs) == 0 {
			return hostAddrs
		}
		return slices.Concat(hostAddrs, appendAddrs)
	}
}

func parseMultiaddrs(strs []string) []ma.Multiaddr {
	addrs := make([]ma.Multiaddr, 0, len(strs))
	for _, s := range strs {
		a, err := ma.NewMultiaddr(s)
		if err != nil {
			log.Errorf("ignoring invalid multiaddr %q: %s", s, err)
			continue
		}
		addrs = append(addrs, a)
	}
	return addrs
}
