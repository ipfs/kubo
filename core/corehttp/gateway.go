package corehttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/ipfs/boxo/blockservice"
	iface "github.com/ipfs/boxo/coreiface"
	"github.com/ipfs/boxo/coreiface/path"
	"github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/gateway"
	"github.com/ipfs/boxo/namesys"
	offlineroute "github.com/ipfs/boxo/routing/offline"
	cid "github.com/ipfs/go-cid"
	version "github.com/ipfs/kubo"
	config "github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node"
	"github.com/libp2p/go-libp2p/core/routing"
	id "github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func GatewayOption(paths ...string) ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}

		headers := make(map[string][]string, len(cfg.Gateway.HTTPHeaders))
		for h, v := range cfg.Gateway.HTTPHeaders {
			headers[http.CanonicalHeaderKey(h)] = v
		}

		gateway.AddAccessControlHeaders(headers)

		gwConfig := gateway.Config{
			Headers: headers,
		}

		gwAPI, err := newGatewayBackend(n)
		if err != nil {
			return nil, err
		}

		gw := gateway.NewHandler(gwConfig, gwAPI)
		// TODO: Add otelhttp.WithPropagators(tracing.Propagator()) option to
		// propagate traces through the gateway once we test this feature.
		gw = otelhttp.NewHandler(gw, "Gateway.Request")

		// By default, our HTTP handler is the gateway handler.
		handler := gw.ServeHTTP

		for _, p := range paths {
			mux.HandleFunc(p+"/", handler)
		}

		return mux, nil
	}
}

func HostnameOption() ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}

		gwAPI, err := newGatewayBackend(n)
		if err != nil {
			return nil, err
		}

		publicGateways := convertPublicGateways(cfg.Gateway.PublicGateways)
		childMux := http.NewServeMux()
		mux.HandleFunc("/", gateway.WithHostname(childMux, gwAPI, publicGateways, cfg.Gateway.NoDNSLink).ServeHTTP)
		return childMux, nil
	}
}

func VersionOption() ServeOption {
	return func(_ *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Commit: %s\n", version.CurrentCommit)
			fmt.Fprintf(w, "Client Version: %s\n", version.GetUserAgentVersion())
			fmt.Fprintf(w, "Protocol Version: %s\n", id.DefaultProtocolVersion)
		})
		return mux, nil
	}
}

func newGatewayBackend(n *core.IpfsNode) (gateway.IPFSBackend, error) {
	cfg, err := n.Repo.Config()
	if err != nil {
		return nil, err
	}

	bserv := n.Blocks
	var vsRouting routing.ValueStore = n.Routing
	nsys := n.Namesys
	if cfg.Gateway.NoFetch {
		bserv = blockservice.New(bserv.Blockstore(), offline.Exchange(bserv.Blockstore()))

		cs := cfg.Ipns.ResolveCacheSize
		if cs == 0 {
			cs = node.DefaultIpnsCacheSize
		}
		if cs < 0 {
			return nil, fmt.Errorf("cannot specify negative resolve cache size")
		}

		vsRouting = offlineroute.NewOfflineRouter(n.Repo.Datastore(), n.RecordValidator)
		nsys, err = namesys.NewNameSystem(vsRouting,
			namesys.WithDatastore(n.Repo.Datastore()),
			namesys.WithDNSResolver(n.DNSResolver),
			namesys.WithCache(cs))
		if err != nil {
			return nil, fmt.Errorf("error constructing namesys: %w", err)
		}
	}

	gw, err := gateway.NewBlocksGateway(bserv, gateway.WithValueStore(vsRouting), gateway.WithNameSystem(nsys))
	if err != nil {
		return nil, err
	}
	return &offlineGatewayErrWrapper{gwimpl: gw}, nil
}

type offlineGatewayErrWrapper struct {
	gwimpl gateway.IPFSBackend
}

func offlineErrWrap(err error) error {
	if errors.Is(err, iface.ErrOffline) {
		return fmt.Errorf("%s : %w", err.Error(), gateway.ErrServiceUnavailable)
	}
	return err
}

func (o *offlineGatewayErrWrapper) Get(ctx context.Context, path gateway.ImmutablePath) (gateway.ContentPathMetadata, *gateway.GetResponse, error) {
	md, n, err := o.gwimpl.Get(ctx, path)
	err = offlineErrWrap(err)
	return md, n, err
}

func (o *offlineGatewayErrWrapper) GetRange(ctx context.Context, path gateway.ImmutablePath, ranges ...gateway.GetRange) (gateway.ContentPathMetadata, files.File, error) {
	md, n, err := o.gwimpl.GetRange(ctx, path, ranges...)
	err = offlineErrWrap(err)
	return md, n, err
}

func (o *offlineGatewayErrWrapper) GetAll(ctx context.Context, path gateway.ImmutablePath) (gateway.ContentPathMetadata, files.Node, error) {
	md, n, err := o.gwimpl.GetAll(ctx, path)
	err = offlineErrWrap(err)
	return md, n, err
}

func (o *offlineGatewayErrWrapper) GetBlock(ctx context.Context, path gateway.ImmutablePath) (gateway.ContentPathMetadata, files.File, error) {
	md, n, err := o.gwimpl.GetBlock(ctx, path)
	err = offlineErrWrap(err)
	return md, n, err
}

func (o *offlineGatewayErrWrapper) Head(ctx context.Context, path gateway.ImmutablePath) (gateway.ContentPathMetadata, files.Node, error) {
	md, n, err := o.gwimpl.Head(ctx, path)
	err = offlineErrWrap(err)
	return md, n, err
}

func (o *offlineGatewayErrWrapper) ResolvePath(ctx context.Context, path gateway.ImmutablePath) (gateway.ContentPathMetadata, error) {
	md, err := o.gwimpl.ResolvePath(ctx, path)
	err = offlineErrWrap(err)
	return md, err
}

func (o *offlineGatewayErrWrapper) GetCAR(ctx context.Context, path gateway.ImmutablePath) (gateway.ContentPathMetadata, io.ReadCloser, <-chan error, error) {
	md, data, errCh, err := o.gwimpl.GetCAR(ctx, path)
	err = offlineErrWrap(err)
	return md, data, errCh, err
}

func (o *offlineGatewayErrWrapper) IsCached(ctx context.Context, path path.Path) bool {
	return o.gwimpl.IsCached(ctx, path)
}

func (o *offlineGatewayErrWrapper) GetIPNSRecord(ctx context.Context, c cid.Cid) ([]byte, error) {
	rec, err := o.gwimpl.GetIPNSRecord(ctx, c)
	err = offlineErrWrap(err)
	return rec, err
}

func (o *offlineGatewayErrWrapper) ResolveMutable(ctx context.Context, path path.Path) (gateway.ImmutablePath, error) {
	imPath, err := o.gwimpl.ResolveMutable(ctx, path)
	err = offlineErrWrap(err)
	return imPath, err
}

func (o *offlineGatewayErrWrapper) GetDNSLinkRecord(ctx context.Context, s string) (path.Path, error) {
	p, err := o.gwimpl.GetDNSLinkRecord(ctx, s)
	err = offlineErrWrap(err)
	return p, err
}

var _ gateway.IPFSBackend = (*offlineGatewayErrWrapper)(nil)

var defaultPaths = []string{"/ipfs/", "/ipns/", "/api/", "/p2p/"}

var subdomainGatewaySpec = &gateway.Specification{
	Paths:         defaultPaths,
	UseSubdomains: true,
}

var defaultKnownGateways = map[string]*gateway.Specification{
	"localhost": subdomainGatewaySpec,
}

func convertPublicGateways(publicGateways map[string]*config.GatewaySpec) map[string]*gateway.Specification {
	gws := map[string]*gateway.Specification{}

	// First, implicit defaults such as subdomain gateway on localhost
	for hostname, gw := range defaultKnownGateways {
		gws[hostname] = gw
	}

	// Then apply values from Gateway.PublicGateways, if present in the config
	for hostname, gw := range publicGateways {
		if gw == nil {
			// Remove any implicit defaults, if present. This is useful when one
			// wants to disable subdomain gateway on localhost etc.
			delete(gws, hostname)
			continue
		}

		gws[hostname] = &gateway.Specification{
			Paths:         gw.Paths,
			NoDNSLink:     gw.NoDNSLink,
			UseSubdomains: gw.UseSubdomains,
			InlineDNSLink: gw.InlineDNSLink.WithDefault(config.DefaultInlineDNSLink),
		}
	}

	return gws
}
