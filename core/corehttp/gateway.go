package corehttp

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/ipfs/go-libipfs/files"
	"github.com/ipfs/go-libipfs/gateway"
	"github.com/ipfs/go-namesys"
	iface "github.com/ipfs/interface-go-ipfs-core"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	nsopts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
	"github.com/ipfs/interface-go-ipfs-core/path"
	version "github.com/ipfs/kubo"
	config "github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	id "github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func GatewayOption(writable bool, paths ...string) ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}

		api, err := coreapi.NewCoreAPI(n, options.Api.FetchBlocks(!cfg.Gateway.NoFetch))
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

		gwAPI, err := newGatewayAPI(n)
		if err != nil {
			return nil, err
		}

		gw := gateway.NewHandler(gwConfig, gwAPI)
		gw = otelhttp.NewHandler(gw, "Gateway.Request")

		// By default, our HTTP handler is the gateway handler.
		handler := gw.ServeHTTP

		// If we have the writable gateway enabled, we have to replace our
		// http handler by a handler that takes care of the different methods.
		if writable {
			writableGw := &writableGatewayHandler{
				config: &gwConfig,
				api:    api,
			}

			handler = func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPost:
					writableGw.postHandler(w, r)
				case http.MethodDelete:
					writableGw.deleteHandler(w, r)
				case http.MethodPut:
					writableGw.putHandler(w, r)
				default:
					gw.ServeHTTP(w, r)
				}
			}
		}

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

		gwAPI, err := newGatewayAPI(n)
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

type gatewayAPI struct {
	ns         namesys.NameSystem
	api        iface.CoreAPI
	offlineAPI iface.CoreAPI
}

func newGatewayAPI(n *core.IpfsNode) (*gatewayAPI, error) {
	cfg, err := n.Repo.Config()
	if err != nil {
		return nil, err
	}

	api, err := coreapi.NewCoreAPI(n, options.Api.FetchBlocks(!cfg.Gateway.NoFetch))
	if err != nil {
		return nil, err
	}
	offlineAPI, err := api.WithOptions(options.Api.Offline(true))
	if err != nil {
		return nil, err
	}

	return &gatewayAPI{
		ns:         n.Namesys,
		api:        api,
		offlineAPI: offlineAPI,
	}, nil
}

func (gw *gatewayAPI) GetUnixFsNode(ctx context.Context, pth path.Resolved) (files.Node, error) {
	return gw.api.Unixfs().Get(ctx, pth)
}

func (gw *gatewayAPI) LsUnixFsDir(ctx context.Context, pth path.Resolved) (<-chan iface.DirEntry, error) {
	// Optimization: use Unixfs.Ls without resolving children, but using the
	// cumulative DAG size as the file size. This allows for a fast listing
	// while keeping a good enough Size field.
	return gw.api.Unixfs().Ls(ctx, pth,
		options.Unixfs.ResolveChildren(false),
		options.Unixfs.UseCumulativeSize(true),
	)
}

func (gw *gatewayAPI) GetBlock(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	r, err := gw.api.Block().Get(ctx, path.IpfsPath(cid))
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, cid)
}

func (gw *gatewayAPI) GetIPNSRecord(ctx context.Context, c cid.Cid) ([]byte, error) {
	return gw.api.Routing().Get(ctx, "/ipns/"+c.String())
}

func (gw *gatewayAPI) GetDNSLinkRecord(ctx context.Context, hostname string) (path.Path, error) {
	p, err := gw.ns.Resolve(ctx, "/ipns/"+hostname, nsopts.Depth(1))
	if err == namesys.ErrResolveRecursion {
		err = nil
	}
	return path.New(p.String()), err
}

func (gw *gatewayAPI) IsCached(ctx context.Context, pth path.Path) bool {
	_, err := gw.offlineAPI.Block().Stat(ctx, pth)
	return err == nil
}

func (gw *gatewayAPI) ResolvePath(ctx context.Context, pth path.Path) (path.Resolved, error) {
	return gw.api.ResolvePath(ctx, pth)
}

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
