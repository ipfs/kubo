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
	iface "github.com/ipfs/interface-go-ipfs-core"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	version "github.com/ipfs/kubo"
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

		offlineAPI, err := api.WithOptions(options.Api.Offline(true))
		if err != nil {
			return nil, err
		}

		gatewayConfig := gateway.Config{
			Headers: headers,
		}

		gatewayAPI := &gatewayAPI{
			api:        api,
			offlineAPI: offlineAPI,
		}

		gateway := gateway.NewHandler(gatewayConfig, gatewayAPI)
		gateway = otelhttp.NewHandler(gateway, "Gateway.Request")

		var writableGateway *writableGatewayHandler
		if writable {
			writableGateway = &writableGatewayHandler{
				config: &gatewayConfig,
				api:    api,
			}
		}

		for _, p := range paths {
			mux.HandleFunc(p+"/", func(w http.ResponseWriter, r *http.Request) {
				if writable {
					switch r.Method {
					case http.MethodPost:
						writableGateway.postHandler(w, r)
					case http.MethodDelete:
						writableGateway.deleteHandler(w, r)
					case http.MethodPut:
						writableGateway.putHandler(w, r)
					default:
						gateway.ServeHTTP(w, r)
					}

					return
				}

				gateway.ServeHTTP(w, r)
			})
		}
		return mux, nil
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
	api        iface.CoreAPI
	offlineAPI iface.CoreAPI
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

func (gw *gatewayAPI) IsCached(ctx context.Context, pth path.Path) bool {
	_, err := gw.offlineAPI.Block().Stat(ctx, pth)
	return err == nil
}

func (gw *gatewayAPI) ResolvePath(ctx context.Context, pth path.Path) (path.Resolved, error) {
	return gw.api.ResolvePath(ctx, pth)
}
