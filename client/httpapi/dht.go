package httpapi

import (
	"context"
	"encoding/json"

	"github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-peerstore"
	notif "github.com/libp2p/go-libp2p-routing/notifications"
)

type DhtAPI HttpApi

func (api *DhtAPI) FindPeer(ctx context.Context, p peer.ID) (peerstore.PeerInfo, error) {
	var out struct {
		Type      notif.QueryEventType
		Responses []peerstore.PeerInfo
	}
	resp, err := api.core().request("dht/findpeer", p.Pretty()).Send(ctx)
	if err != nil {
		return peerstore.PeerInfo{}, err
	}
	if resp.Error != nil {
		return peerstore.PeerInfo{}, resp.Error
	}
	defer resp.Close()
	dec := json.NewDecoder(resp.Output)
	for {
		if err := dec.Decode(&out); err != nil {
			return peerstore.PeerInfo{}, err
		}
		if out.Type == notif.FinalPeer {
			return out.Responses[0], nil
		}
	}
}

func (api *DhtAPI) FindProviders(ctx context.Context, p iface.Path, opts ...caopts.DhtFindProvidersOption) (<-chan peerstore.PeerInfo, error) {
	options, err := caopts.DhtFindProvidersOptions(opts...)
	if err != nil {
		return nil, err
	}

	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	resp, err := api.core().request("dht/findprovs", rp.Cid().String()).
		Option("num-providers", options.NumProviders).
		Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	res := make(chan peerstore.PeerInfo)

	go func() {
		defer resp.Close()
		defer close(res)
		dec := json.NewDecoder(resp.Output)

		for {
			var out struct {
				Extra     string
				Type      notif.QueryEventType
				Responses []peerstore.PeerInfo
			}

			if err := dec.Decode(&out); err != nil {
				return // todo: handle this somehow
			}
			if out.Type == notif.QueryError {
				return // usually a 'not found' error
				// todo: handle other errors
			}
			if out.Type == notif.Provider {
				for _, pi := range out.Responses {
					select {
					case res <- pi:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return res, nil
}

func (api *DhtAPI) Provide(ctx context.Context, p iface.Path, opts ...caopts.DhtProvideOption) error {
	options, err := caopts.DhtProvideOptions(opts...)
	if err != nil {
		return err
	}

	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return err
	}

	return api.core().request("dht/provide", rp.Cid().String()).
		Option("recursive", options.Recursive).
		Exec(ctx, nil)
}

func (api *DhtAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
