package routing

import (
	"context"
	"errors"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-cid"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multihash"
)

var (
	_ routinghelpers.ProvideManyRouter = &Composer{}
	_ routing.Routing                  = &Composer{}
)

type Composer struct {
	GetValueRouter      routing.Routing
	PutValueRouter      routing.Routing
	FindPeersRouter     routing.Routing
	FindProvidersRouter routing.Routing
	ProvideRouter       routing.Routing
}

func (c *Composer) Provide(ctx context.Context, cid cid.Cid, provide bool) error {
	log.Debug("composer: calling provide: ", cid)
	err := c.ProvideRouter.Provide(ctx, cid, provide)
	if err != nil {
		log.Debug("composer: calling provide: ", cid, " error: ", err)
	}

	return err
}

func (c *Composer) ProvideMany(ctx context.Context, keys []multihash.Multihash) error {
	log.Debug("composer: calling provide many: ", len(keys))
	pmr, ok := c.ProvideRouter.(routinghelpers.ProvideManyRouter)
	if !ok {
		log.Debug("composer: provide many is not implemented on the actual router")
		return nil
	}

	err := pmr.ProvideMany(ctx, keys)
	if err != nil {
		log.Debug("composer: calling provide many error: ", err)
	}

	return err
}

func (c *Composer) Ready() bool {
	log.Debug("composer: calling ready")
	pmr, ok := c.ProvideRouter.(routinghelpers.ReadyAbleRouter)
	if !ok {
		return true
	}

	ready := pmr.Ready()

	log.Debug("composer: calling ready result: ", ready)

	return ready
}

func (c *Composer) FindProvidersAsync(ctx context.Context, cid cid.Cid, count int) <-chan peer.AddrInfo {
	log.Debug("composer: calling findProvidersAsync: ", cid)
	return c.FindProvidersRouter.FindProvidersAsync(ctx, cid, count)
}

func (c *Composer) FindPeer(ctx context.Context, pid peer.ID) (peer.AddrInfo, error) {
	log.Debug("composer: calling findPeer: ", pid)
	addr, err := c.FindPeersRouter.FindPeer(ctx, pid)
	if err != nil {
		log.Debug("composer: calling findPeer error: ", pid, addr.String(), err)
	}
	return addr, err
}

func (c *Composer) PutValue(ctx context.Context, key string, val []byte, opts ...routing.Option) error {
	log.Debug("composer: calling putValue: ", key, len(val))
	err := c.PutValueRouter.PutValue(ctx, key, val, opts...)
	if err != nil {
		log.Debug("composer: calling putValue error: ", key, len(val), err)
	}

	return err
}

func (c *Composer) GetValue(ctx context.Context, key string, opts ...routing.Option) ([]byte, error) {
	log.Debug("composer: calling getValue: ", key)
	val, err := c.GetValueRouter.GetValue(ctx, key, opts...)
	if err != nil {
		log.Debug("composer: calling getValue error: ", key, len(val), err)
	}

	return val, err
}

func (c *Composer) SearchValue(ctx context.Context, key string, opts ...routing.Option) (<-chan []byte, error) {
	log.Debug("composer: calling searchValue: ", key)
	ch, err := c.GetValueRouter.SearchValue(ctx, key, opts...)

	// avoid nil channels on implementations not supporting SearchValue method.
	if errors.Is(err, routing.ErrNotFound) && ch == nil {
		out := make(chan []byte)
		close(out)
		return out, err
	}

	if err != nil {
		log.Debug("composer: calling searchValue error: ", key, err)
	}

	return ch, err
}

func (c *Composer) Bootstrap(ctx context.Context) error {
	log.Debug("composer: calling bootstrap")
	errfp := c.FindPeersRouter.Bootstrap(ctx)
	errfps := c.FindProvidersRouter.Bootstrap(ctx)
	errgv := c.GetValueRouter.Bootstrap(ctx)
	errpv := c.PutValueRouter.Bootstrap(ctx)
	errp := c.ProvideRouter.Bootstrap(ctx)
	err := multierror.Append(errfp, errfps, errgv, errpv, errp)
	if err != nil {
		log.Debug("composer: calling bootstrap error: ", err)
	}
	return err
}
