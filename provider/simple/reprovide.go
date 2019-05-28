package simple

import (
	"context"
	"fmt"
	"time"

	backoff "github.com/cenkalti/backoff"
	cid "github.com/ipfs/go-cid"
	cidutil "github.com/ipfs/go-cidutil"
	blocks "github.com/ipfs/go-ipfs-blockstore"
	pin "github.com/ipfs/go-ipfs/pin"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	merkledag "github.com/ipfs/go-merkledag"
	verifcid "github.com/ipfs/go-verifcid"
	routing "github.com/libp2p/go-libp2p-core/routing"
)

var logR = logging.Logger("reprovider.simple")

//KeyChanFunc is function streaming CIDs to pass to content routing
type KeyChanFunc func(context.Context) (<-chan cid.Cid, error)
type doneFunc func(error)

// Reprovider reannounces blocks to the network
type Reprovider struct {
	ctx     context.Context
	trigger chan doneFunc

	// The routing system to provide values through
	rsys routing.ContentRouting

	keyProvider KeyChanFunc

	tick time.Duration
}

// NewReprovider creates new Reprovider instance.
func NewReprovider(ctx context.Context, reprovideIniterval time.Duration, rsys routing.ContentRouting, keyProvider KeyChanFunc) *Reprovider {
	return &Reprovider{
		ctx:     ctx,
		trigger: make(chan doneFunc),

		rsys:        rsys,
		keyProvider: keyProvider,
		tick:        reprovideIniterval,
	}
}

// Close the reprovider
func (rp *Reprovider) Close() error {
	return nil
}

// Run re-provides keys with 'tick' interval or when triggered
func (rp *Reprovider) Run() {
	// dont reprovide immediately.
	// may have just started the daemon and shutting it down immediately.
	// probability( up another minute | uptime ) increases with uptime.
	after := time.After(time.Minute)
	var done doneFunc
	for {
		if rp.tick == 0 {
			after = make(chan time.Time)
		}

		select {
		case <-rp.ctx.Done():
			return
		case done = <-rp.trigger:
		case <-after:
		}

		//'mute' the trigger channel so when `ipfs bitswap reprovide` is called
		//a 'reprovider is already running' error is returned
		unmute := rp.muteTrigger()

		err := rp.Reprovide()
		if err != nil {
			logR.Debug(err)
		}

		if done != nil {
			done(err)
		}

		unmute()

		after = time.After(rp.tick)
	}
}

// Reprovide registers all keys given by rp.keyProvider to libp2p content routing
func (rp *Reprovider) Reprovide() error {
	keychan, err := rp.keyProvider(rp.ctx)
	if err != nil {
		return fmt.Errorf("failed to get key chan: %s", err)
	}
	for c := range keychan {
		// hash security
		if err := verifcid.ValidateCid(c); err != nil {
			logR.Errorf("insecure hash in reprovider, %s (%s)", c, err)
			continue
		}
		op := func() error {
			err := rp.rsys.Provide(rp.ctx, c, true)
			if err != nil {
				logR.Debugf("Failed to provide key: %s", err)
			}
			return err
		}

		// TODO: this backoff library does not respect our context, we should
		// eventually work contexts into it. low priority.
		err := backoff.Retry(op, backoff.NewExponentialBackOff())
		if err != nil {
			logR.Debugf("Providing failed after number of retries: %s", err)
			return err
		}
	}
	return nil
}

// Trigger starts reprovision process in rp.Run and waits for it
func (rp *Reprovider) Trigger(ctx context.Context) error {
	progressCtx, done := context.WithCancel(ctx)

	var err error
	df := func(e error) {
		err = e
		done()
	}

	select {
	case <-rp.ctx.Done():
		return context.Canceled
	case <-ctx.Done():
		return context.Canceled
	case rp.trigger <- df:
		<-progressCtx.Done()
		return err
	}
}

func (rp *Reprovider) muteTrigger() context.CancelFunc {
	ctx, cf := context.WithCancel(rp.ctx)
	go func() {
		defer cf()
		for {
			select {
			case <-ctx.Done():
				return
			case done := <-rp.trigger:
				done(fmt.Errorf("reprovider is already running"))
			}
		}
	}()

	return cf
}

// Strategies

// NewBlockstoreProvider returns key provider using bstore.AllKeysChan
func NewBlockstoreProvider(bstore blocks.Blockstore) KeyChanFunc {
	return func(ctx context.Context) (<-chan cid.Cid, error) {
		return bstore.AllKeysChan(ctx)
	}
}

// NewPinnedProvider returns provider supplying pinned keys
func NewPinnedProvider(onlyRoots bool) func(pin.Pinner, ipld.DAGService) KeyChanFunc {
	return func(pinning pin.Pinner, dag ipld.DAGService) KeyChanFunc {
		return func(ctx context.Context) (<-chan cid.Cid, error) {
			set, err := pinSet(ctx, pinning, dag, onlyRoots)
			if err != nil {
				return nil, err
			}

			outCh := make(chan cid.Cid)
			go func() {
				defer close(outCh)
				for c := range set.New {
					select {
					case <-ctx.Done():
						return
					case outCh <- c:
					}
				}

			}()

			return outCh, nil
		}
	}
}

func pinSet(ctx context.Context, pinning pin.Pinner, dag ipld.DAGService, onlyRoots bool) (*cidutil.StreamingSet, error) {
	set := cidutil.NewStreamingSet()

	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer close(set.New)

		for _, key := range pinning.DirectKeys() {
			set.Visitor(ctx)(key)
		}

		for _, key := range pinning.RecursiveKeys() {
			set.Visitor(ctx)(key)

			if !onlyRoots {
				err := merkledag.EnumerateChildren(ctx, merkledag.GetLinksWithDAG(dag), key, set.Visitor(ctx))
				if err != nil {
					logR.Errorf("reprovide indirect pins: %s", err)
					return
				}
			}
		}
	}()

	return set, nil
}
