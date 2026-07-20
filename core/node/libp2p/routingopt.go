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
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
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

// httpRouterAddrFunc returns a function that resolves provider addresses for
// HTTP routers at provide-time.
//
// Resolution logic:
//   - If Announce is set, use it as a static override (no dynamic resolution).
//   - Otherwise announce host.Addrs(), the same set identify sends to peers:
//     0.0.0.0/:: Swarm binds resolved to concrete interface addresses, the
//     libp2p AddrsFactory applied (Addresses.NoAnnounce, and the AutoTLS
//     /tls/ws address, which only exists here), and certhashes attached.
//   - Narrowed to globally routable addresses when the node has any, matching
//     the DHT provide path (selfAddrsFunc in core/node/provider.go). A node
//     with no public address keeps announcing what it has, so LAN-only setups
//     pointing at a local router still publish something dialable.
//   - AppendAnnounce addresses are always appended, exactly once, and are
//     exempt from both NoAnnounce and the public-address narrowing: an
//     explicit operator entry wins.
//
// Do not narrow this to AutoNAT V2 confirmed reachable addresses. AutoNAT only
// ever sees listen addresses, so an address the AddrsFactory synthesizes (the
// AutoTLS /tls/ws one) can never be confirmed, and browser clients that reach
// us through a delegated router are left with nothing they can dial.
// See https://github.com/ipfs/kubo/issues/11369.
func httpRouterAddrFunc(h host.Host, cfgAddrs config.Addresses) func() []ma.Multiaddr {
	appendAddrs := parseMultiaddrs(cfgAddrs.AppendAnnounce)

	// If Announce is explicitly set, use it as a static override.
	if len(cfgAddrs.Announce) > 0 {
		announceAddrs := parseMultiaddrs(cfgAddrs.Announce)
		// Skip AppendAnnounce entries already listed in Announce,
		// mirroring makeAddrsFactory (addrs.go).
		extra := ma.FilterAddrs(appendAddrs, func(a ma.Multiaddr) bool {
			return !ma.Contains(announceAddrs, a)
		})
		staticAddrs := slices.Concat(announceAddrs, extra)
		return func() []ma.Multiaddr { return staticAddrs }
	}

	return func() []ma.Multiaddr {
		addrs := h.Addrs()
		if len(appendAddrs) > 0 {
			// The AddrsFactory (makeAddrsFactory in addrs.go) already injects
			// AppendAnnounce into h.Addrs(). Take those out so the re-append
			// below emits them exactly once, and so a public AppendAnnounce
			// entry cannot trip the public-addr guard on a node whose own
			// addresses are all private.
			addrs = ma.FilterAddrs(addrs, func(a ma.Multiaddr) bool {
				return !ma.Contains(appendAddrs, a)
			})
		}
		// Delegated routers are public indexes, so keep loopback and LAN
		// addresses out of the record whenever we have a routable one.
		if public := ma.FilterAddrs(addrs, manet.IsPublicAddr); len(public) > 0 {
			addrs = public
		}
		if len(appendAddrs) == 0 {
			return addrs
		}
		return slices.Concat(addrs, appendAddrs)
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
