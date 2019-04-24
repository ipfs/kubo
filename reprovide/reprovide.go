package reprovide

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/go-verifcid"
	"github.com/jbenet/goprocess"
	goprocessctx "github.com/jbenet/goprocess/context"
	routing "github.com/libp2p/go-libp2p-routing"
)

var log = logging.Logger("reprovider")

// KeyChanFunc is function streaming CIDs to pass to content routing
type KeyChanFunc func(context.Context) (<-chan cid.Cid, error)
type doneFunc func(error)

type Reprovider struct {
	ctx     context.Context
	trigger chan doneFunc
	closing chan struct{}

	// The routing system to provide values through
	rsys routing.ContentRouting

	keyProvider KeyChanFunc
	tick        time.Duration
}

// NewReprovider creates new Reprovider instance.
func NewReprovider(ctx context.Context, tick time.Duration, rsys routing.ContentRouting, keyProvider KeyChanFunc) *Reprovider {
	return &Reprovider{
		ctx:     ctx,
		trigger: make(chan doneFunc),
		closing: make(chan struct{}),

		rsys:        rsys,
		keyProvider: keyProvider,
		tick:        tick,
	}
}

// Run re-provides keys with 'tick' interval or when triggered
func (rp *Reprovider) Run(proc goprocess.Process) {
	ctx := goprocessctx.WithProcessClosing(rp.ctx, proc)
	defer close(rp.closing)

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
		case <-ctx.Done():
			return
		case done = <-rp.trigger:
		case <-after:
		}

		// 'mute' the trigger channel so when `ipfs bitswap reprovide` is called
		// a 'reprovider is already running' error is returned
		unmute := rp.muteTrigger()

		err := rp.reprovide(ctx)
		if err != nil {
			log.Debug(err)
		}

		if done != nil {
			done(err)
		}

		unmute()

		after = time.After(rp.tick)
	}
}

// reprovide registers all keys given by rp.keyProvider to libp2p content routing
func (rp *Reprovider) reprovide(ctx context.Context) error {
	keychan, err := rp.keyProvider(ctx)
	if err != nil {
		return fmt.Errorf("failed to get key chan: %s", err)
	}
	for c := range keychan {
		// hash security
		if err := verifcid.ValidateCid(c); err != nil {
			log.Errorf("insecure hash in reprovider, %s (%s)", c, err)
			continue
		}
		op := func() error {
			err := rp.rsys.Provide(ctx, c, true)
			if err != nil {
				log.Debugf("Failed to provide key: %s", err)
			}
			return err
		}

		// TODO: this backoff library does not respect our context, we should
		// eventually work contexts into it. low priority.
		err := backoff.Retry(op, backoff.NewExponentialBackOff())
		if err != nil {
			log.Debugf("Providing failed after number of retries: %s", err)
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
	case <-rp.closing:
		return errors.New("reprovider is closed")
	case <-rp.ctx.Done():
		return rp.ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
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
