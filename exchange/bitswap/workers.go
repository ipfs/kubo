package bitswap

import (
	"time"

	process "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	procctx "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/context"
	ratelimit "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/ratelimit"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	logging "github.com/ipfs/go-ipfs/vendor/go-log-v1.0.0"
)

var TaskWorkerCount = 8

func (bs *Bitswap) startWorkers(px process.Process, ctx context.Context) {
	// Start up a worker to handle block requests this node is making
	px.Go(func(px process.Process) {
		bs.providerConnector(ctx)
	})

	// Start up workers to handle requests from other nodes for the data on this node
	for i := 0; i < TaskWorkerCount; i++ {
		i := i
		px.Go(func(px process.Process) {
			bs.taskWorker(ctx, i)
		})
	}

	// Start up a worker to manage periodically resending our wantlist out to peers
	px.Go(func(px process.Process) {
		bs.rebroadcastWorker(ctx)
	})

	// Start up a worker to manage sending out provides messages
	px.Go(func(px process.Process) {
		bs.provideCollector(ctx)
	})

	// Spawn up multiple workers to handle incoming blocks
	// consider increasing number if providing blocks bottlenecks
	// file transfers
	px.Go(bs.provideWorker)
}

func (bs *Bitswap) taskWorker(ctx context.Context, id int) {
	idmap := logging.LoggableMap{"ID": id}
	defer log.Info("bitswap task worker shutting down...")
	for {
		log.Event(ctx, "Bitswap.TaskWorker.Loop", idmap)
		select {
		case nextEnvelope := <-bs.engine.Outbox():
			select {
			case envelope, ok := <-nextEnvelope:
				if !ok {
					continue
				}
				log.Event(ctx, "Bitswap.TaskWorker.Work", logging.LoggableMap{
					"ID":     id,
					"Target": envelope.Peer.Pretty(),
					"Block":  envelope.Block.Multihash.B58String(),
				})

				bs.wm.SendBlock(ctx, envelope)
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (bs *Bitswap) provideWorker(px process.Process) {

	limiter := ratelimit.NewRateLimiter(px, provideWorkerMax)

	limitedGoProvide := func(k key.Key, wid int) {
		ev := logging.LoggableMap{"ID": wid}
		limiter.LimitedGo(func(px process.Process) {

			ctx := procctx.OnClosingContext(px) // derive ctx from px
			defer log.EventBegin(ctx, "Bitswap.ProvideWorker.Work", ev, &k).Done()

			ctx, cancel := context.WithTimeout(ctx, provideTimeout) // timeout ctx
			defer cancel()

			if err := bs.network.Provide(ctx, k); err != nil {
				log.Error(err)
			}
		})
	}

	// worker spawner, reads from bs.provideKeys until it closes, spawning a
	// _ratelimited_ number of workers to handle each key.
	limiter.Go(func(px process.Process) {
		for wid := 2; ; wid++ {
			ev := logging.LoggableMap{"ID": 1}
			log.Event(procctx.OnClosingContext(px), "Bitswap.ProvideWorker.Loop", ev)

			select {
			case <-px.Closing():
				return
			case k, ok := <-bs.provideKeys:
				if !ok {
					log.Debug("provideKeys channel closed")
					return
				}
				limitedGoProvide(k, wid)
			}
		}
	})
}

func (bs *Bitswap) provideCollector(ctx context.Context) {
	defer close(bs.provideKeys)
	var toProvide []key.Key
	var nextKey key.Key
	var keysOut chan key.Key

	for {
		select {
		case blk, ok := <-bs.newBlocks:
			if !ok {
				log.Debug("newBlocks channel closed")
				return
			}
			if keysOut == nil {
				nextKey = blk.Key()
				keysOut = bs.provideKeys
			} else {
				toProvide = append(toProvide, blk.Key())
			}
		case keysOut <- nextKey:
			if len(toProvide) > 0 {
				nextKey = toProvide[0]
				toProvide = toProvide[1:]
			} else {
				keysOut = nil
			}
		case <-ctx.Done():
			return
		}
	}
}

// connects to providers for the given keys
func (bs *Bitswap) providerConnector(parent context.Context) {
	defer log.Info("bitswap client worker shutting down...")

	for {
		log.Event(parent, "Bitswap.ProviderConnector.Loop")
		select {
		case req := <-bs.findKeys:
			keys := req.keys
			if len(keys) == 0 {
				log.Warning("Received batch request for zero blocks")
				continue
			}
			log.Event(parent, "Bitswap.ProviderConnector.Work", logging.LoggableMap{"Keys": keys})

			// NB: Optimization. Assumes that providers of key[0] are likely to
			// be able to provide for all keys. This currently holds true in most
			// every situation. Later, this assumption may not hold as true.
			child, cancel := context.WithTimeout(req.ctx, providerRequestTimeout)
			providers := bs.network.FindProvidersAsync(child, keys[0], maxProvidersPerRequest)
			for p := range providers {
				go bs.network.ConnectTo(req.ctx, p)
			}
			cancel()

		case <-parent.Done():
			return
		}
	}
}

func (bs *Bitswap) rebroadcastWorker(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	broadcastSignal := time.NewTicker(rebroadcastDelay.Get())
	defer broadcastSignal.Stop()

	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()

	for {
		log.Event(ctx, "Bitswap.Rebroadcast.idle")
		select {
		case <-tick.C:
			n := bs.wm.wl.Len()
			if n > 0 {
				log.Debug(n, "keys in bitswap wantlist")
			}
		case <-broadcastSignal.C: // resend unfulfilled wantlist keys
			log.Event(ctx, "Bitswap.Rebroadcast.active")
			entries := bs.wm.wl.Entries()
			if len(entries) > 0 {
				bs.connectToProviders(ctx, entries)
			}
		case <-parent.Done():
			return
		}
	}
}
