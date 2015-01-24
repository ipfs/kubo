package reprovide

import (
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	backoff "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/cenkalti/backoff"

	blocks "github.com/jbenet/go-ipfs/blocks/blockstore"
	routing "github.com/jbenet/go-ipfs/routing"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

var log = eventlog.Logger("reprovider")

type Reprovider struct {
	// The routing system to provide values through
	rsys routing.IpfsRouting

	// The backing store for blocks to be provided
	bstore blocks.Blockstore
}

func NewReprovider(rsys routing.IpfsRouting, bstore blocks.Blockstore) *Reprovider {
	return &Reprovider{
		rsys:   rsys,
		bstore: bstore,
	}
}

func (rp *Reprovider) ProvideEvery(ctx context.Context, tick time.Duration) {
	// dont reprovide immediately.
	// may have just started the daemon and shutting it down immediately.
	// probability( up another minute | uptime ) increases with uptime.
	after := time.After(time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-after:
			err := rp.Reprovide(ctx)
			if err != nil {
				log.Error(err)
			}
			after = time.After(tick)
		}
	}
}

func (rp *Reprovider) Reprovide(ctx context.Context) error {
	keychan, err := rp.bstore.AllKeysChan(ctx)
	if err != nil {
		return debugerror.Errorf("Failed to get key chan from blockstore: %s", err)
	}
	for k := range keychan {
		op := func() error {
			err := rp.rsys.Provide(ctx, k)
			if err != nil {
				log.Warningf("Failed to provide key: %s", err)
			}
			return err
		}

		// TODO: this backoff library does not respect our context, we should
		// eventually work contexts into it. low priority.
		err := backoff.Retry(op, backoff.NewExponentialBackOff())
		if err != nil {
			log.Errorf("Providing failed after number of retries: %s", err)
			return err
		}
	}
	return nil
}
