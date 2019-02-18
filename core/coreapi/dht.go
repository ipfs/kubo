package coreapi

import (
	"context"
	"fmt"

	coreiface "gx/ipfs/QmNt8iUv3uJoVrJ3Ls4cgMLk124V4Bt1JxuUMWbjULt9ns/interface-go-ipfs-core"
	caopts "gx/ipfs/QmNt8iUv3uJoVrJ3Ls4cgMLk124V4Bt1JxuUMWbjULt9ns/interface-go-ipfs-core/options"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	pstore "gx/ipfs/QmRhFARzTHcFh8wUxwN5KvyTGq73FLC65EfFAhz8Ng7aGb/go-libp2p-peerstore"
	blockstore "gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	offline "gx/ipfs/QmSz8kAe2JCKp2dWSG8gHSWnwSmne8YfRXTeK5HBmc9L7t/go-ipfs-exchange-offline"
	peer "gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	routing "gx/ipfs/QmWaDSNoSdSXU9b6udyaq9T8y6LkzMwqWxECznFqvtcTsk/go-libp2p-routing"
	blockservice "gx/ipfs/QmZsGVGCqMCNzHLNMB6q4F6yyvomqf1VxwhJwSfgo1NGaF/go-blockservice"
	dag "gx/ipfs/Qmccmovpo9isKeaaDzcxvT7mVJN1uKwn2xzSs1y8hb6PEs/go-merkledag"
	cidutil "gx/ipfs/QmdPQx9fvN5ExVwMhRmh7YpCQJzJrFhd1AjVBwJmRMFJeX/go-cidutil"
)

type DhtAPI CoreAPI

func (api *DhtAPI) FindPeer(ctx context.Context, p peer.ID) (pstore.PeerInfo, error) {
	err := api.checkOnline(false)
	if err != nil {
		return pstore.PeerInfo{}, err
	}

	pi, err := api.routing.FindPeer(ctx, peer.ID(p))
	if err != nil {
		return pstore.PeerInfo{}, err
	}

	return pi, nil
}

func (api *DhtAPI) FindProviders(ctx context.Context, p coreiface.Path, opts ...caopts.DhtFindProvidersOption) (<-chan pstore.PeerInfo, error) {
	settings, err := caopts.DhtFindProvidersOptions(opts...)
	if err != nil {
		return nil, err
	}

	err = api.checkOnline(false)
	if err != nil {
		return nil, err
	}

	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	numProviders := settings.NumProviders
	if numProviders < 1 {
		return nil, fmt.Errorf("number of providers must be greater than 0")
	}

	pchan := api.routing.FindProvidersAsync(ctx, rp.Cid(), numProviders)
	return pchan, nil
}

func (api *DhtAPI) Provide(ctx context.Context, path coreiface.Path, opts ...caopts.DhtProvideOption) error {
	settings, err := caopts.DhtProvideOptions(opts...)
	if err != nil {
		return err
	}

	err = api.checkOnline(false)
	if err != nil {
		return err
	}

	rp, err := api.core().ResolvePath(ctx, path)
	if err != nil {
		return err
	}

	c := rp.Cid()

	has, err := api.blockstore.Has(c)
	if err != nil {
		return err
	}

	if !has {
		return fmt.Errorf("block %s not found locally, cannot provide", c)
	}

	if settings.Recursive {
		err = provideKeysRec(ctx, api.routing, api.blockstore, []cid.Cid{c})
	} else {
		err = provideKeys(ctx, api.routing, []cid.Cid{c})
	}
	if err != nil {
		return err
	}

	return nil
}

func provideKeys(ctx context.Context, r routing.IpfsRouting, cids []cid.Cid) error {
	for _, c := range cids {
		err := r.Provide(ctx, c, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func provideKeysRec(ctx context.Context, r routing.IpfsRouting, bs blockstore.Blockstore, cids []cid.Cid) error {
	provided := cidutil.NewStreamingSet()

	errCh := make(chan error)
	go func() {
		dserv := dag.NewDAGService(blockservice.New(bs, offline.Exchange(bs)))
		for _, c := range cids {
			err := dag.EnumerateChildrenAsync(ctx, dag.GetLinksDirect(dserv), c, provided.Visitor(ctx))
			if err != nil {
				errCh <- err
			}
		}
	}()

	for {
		select {
		case k := <-provided.New:
			err := r.Provide(ctx, k, true)
			if err != nil {
				return err
			}
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (api *DhtAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
