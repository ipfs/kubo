package coreapi

import (
	"context"
	"errors"
	"fmt"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	dag "gx/ipfs/QmNr4E8z9bGTztvHJktp7uQaMdx9p3r9Asrq6eYk7iCh4a/go-merkledag"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	ipdht "gx/ipfs/QmRNxiPpZf3skMAtmDJpgHuW9uj1ukqV1zjANj9d6bmHfE/go-libp2p-kad-dht"
	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	routing "gx/ipfs/QmY9JUvS8kbgao3XbPh6WAV3ChE2nxGKhcGTHiwMC4gmcU/go-libp2p-routing"
	notif "gx/ipfs/QmY9JUvS8kbgao3XbPh6WAV3ChE2nxGKhcGTHiwMC4gmcU/go-libp2p-routing/notifications"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	pstore "gx/ipfs/Qmda4cPRvSRyox3SqgJN6DfSZGU5TtHufPTp9uXjFj71X6/go-libp2p-peerstore"
)

var ErrNotDHT = errors.New("routing service is not a DHT")

type DhtAPI struct {
	*CoreAPI
	*caopts.DhtOptions
}

func (api *DhtAPI) FindPeer(ctx context.Context, p peer.ID) (<-chan ma.Multiaddr, error) {
	dht, ok := api.node.Routing.(*ipdht.IpfsDHT)
	if !ok {
		return nil, ErrNotDHT
	}

	outChan := make(chan ma.Multiaddr)
	events := make(chan *notif.QueryEvent)
	ctx = notif.RegisterForQueryEvents(ctx, events)

	go func() {
		defer close(outChan)

		sendAddrs := func(responses []*pstore.PeerInfo) error {
			for _, response := range responses {
				for _, addr := range response.Addrs {
					select {
					case outChan <- addr:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
			return nil
		}

		for event := range events {
			if event.Type == notif.FinalPeer {
				err := sendAddrs(event.Responses)
				if err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer close(events)
		pi, err := dht.FindPeer(ctx, peer.ID(p))
		if err != nil {
			notif.PublishQueryEvent(ctx, &notif.QueryEvent{
				Type:  notif.QueryError,
				Extra: err.Error(),
			})
			return
		}

		notif.PublishQueryEvent(ctx, &notif.QueryEvent{
			Type:      notif.FinalPeer,
			Responses: []*pstore.PeerInfo{&pi},
		})
	}()

	return outChan, nil
}

func (api *DhtAPI) FindProviders(ctx context.Context, p coreiface.Path, opts ...caopts.DhtFindProvidersOption) (<-chan peer.ID, error) {
	settings, err := caopts.DhtFindProvidersOptions(opts...)
	if err != nil {
		return nil, err
	}

	dht, ok := api.node.Routing.(*ipdht.IpfsDHT)
	if !ok {
		return nil, ErrNotDHT
	}

	rp, err := api.ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	c := rp.Cid()

	numProviders := settings.NumProviders
	if numProviders < 1 {
		return nil, fmt.Errorf("number of providers must be greater than 0")
	}

	outChan := make(chan peer.ID)
	events := make(chan *notif.QueryEvent)
	ctx = notif.RegisterForQueryEvents(ctx, events)

	pchan := dht.FindProvidersAsync(ctx, c, numProviders)
	go func() {
		defer close(outChan)

		sendProviders := func(responses []*pstore.PeerInfo) error {
			for _, response := range responses {
				select {
				case outChan <- response.ID:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		}

		for event := range events {
			if event.Type == notif.Provider {
				err := sendProviders(event.Responses)
				if err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer close(events)
		for p := range pchan {
			np := p
			notif.PublishQueryEvent(ctx, &notif.QueryEvent{
				Type:      notif.Provider,
				Responses: []*pstore.PeerInfo{&np},
			})
		}
	}()

	return outChan, nil
}

func (api *DhtAPI) Provide(ctx context.Context, path coreiface.Path, opts ...caopts.DhtProvideOption) error {
	settings, err := caopts.DhtProvideOptions(opts...)
	if err != nil {
		return err
	}

	if api.node.Routing == nil {
		return errors.New("cannot provide in offline mode")
	}

	if len(api.node.PeerHost.Network().Conns()) == 0 {
		return errors.New("cannot provide, no connected peers")
	}

	rp, err := api.ResolvePath(ctx, path)
	if err != nil {
		return err
	}

	c := rp.Cid()

	has, err := api.node.Blockstore.Has(c)
	if err != nil {
		return err
	}

	if !has {
		return fmt.Errorf("block %s not found locally, cannot provide", c)
	}

	//TODO: either remove or use
	//outChan := make(chan interface{})

	//events := make(chan *notif.QueryEvent)
	//ctx = notif.RegisterForQueryEvents(ctx, events)

	/*go func() {
		defer close(outChan)
		for range events {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()*/

	//defer close(events)
	if settings.Recursive {
		err = provideKeysRec(ctx, api.node.Routing, api.node.DAG, []*cid.Cid{c})
	} else {
		err = provideKeys(ctx, api.node.Routing, []*cid.Cid{c})
	}
	if err != nil {
		return err
	}

	return nil
}

func provideKeys(ctx context.Context, r routing.IpfsRouting, cids []*cid.Cid) error {
	for _, c := range cids {
		err := r.Provide(ctx, c, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func provideKeysRec(ctx context.Context, r routing.IpfsRouting, dserv ipld.DAGService, cids []*cid.Cid) error {
	provided := cid.NewSet()
	for _, c := range cids {
		kset := cid.NewSet()

		err := dag.EnumerateChildrenAsync(ctx, dag.GetLinksDirect(dserv), c, kset.Visit)
		if err != nil {
			return err
		}

		for _, k := range kset.Keys() {
			if provided.Has(k) {
				continue
			}

			err = r.Provide(ctx, k, true)
			if err != nil {
				return err
			}
			provided.Add(k)
		}
	}

	return nil
}
