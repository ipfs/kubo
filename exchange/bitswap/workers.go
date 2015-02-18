package bitswap

import (
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	inflect "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/briantigerchow/inflect"
	process "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
)

func (bs *bitswap) startWorkers(px process.Process, ctx context.Context) {
	// Start up a worker to handle block requests this node is making
	px.Go(func(px process.Process) {
		bs.clientWorker(ctx)
	})

	// Start up a worker to handle requests from other nodes for the data on this node
	px.Go(func(px process.Process) {
		bs.taskWorker(ctx)
	})

	// Start up a worker to manage periodically resending our wantlist out to peers
	px.Go(func(px process.Process) {
		bs.rebroadcastWorker(ctx)
	})

	// Spawn up multiple workers to handle incoming blocks
	// consider increasing number if providing blocks bottlenecks
	// file transfers
	for i := 0; i < provideWorkers; i++ {
		px.Go(func(px process.Process) {
			bs.blockReceiveWorker(ctx)
		})
	}
}

func (bs *bitswap) taskWorker(ctx context.Context) {
	defer log.Info("bitswap task worker shutting down...")
	for {
		select {
		case nextEnvelope := <-bs.engine.Outbox():
			select {
			case envelope, ok := <-nextEnvelope:
				if !ok {
					continue
				}
				log.Event(ctx, "deliverBlocks", envelope.Message, envelope.Peer)
				bs.send(ctx, envelope.Peer, envelope.Message)
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (bs *bitswap) blockReceiveWorker(ctx context.Context) {
	for {
		select {
		case blk, ok := <-bs.newBlocks:
			if !ok {
				log.Debug("newBlocks channel closed")
				return
			}
			ctx, _ := context.WithTimeout(ctx, provideTimeout)
			err := bs.network.Provide(ctx, blk.Key())
			if err != nil {
				log.Error(err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// TODO ensure only one active request per key
func (bs *bitswap) clientWorker(parent context.Context) {
	defer log.Info("bitswap client worker shutting down...")

	for {
		select {
		case req := <-bs.batchRequests:
			keys := req.keys
			if len(keys) == 0 {
				log.Warning("Received batch request for zero blocks")
				continue
			}
			for i, k := range keys {
				bs.wantlist.Add(k, kMaxPriority-i)
			}

			bs.wantNewBlocks(req.ctx, keys)

			// NB: Optimization. Assumes that providers of key[0] are likely to
			// be able to provide for all keys. This currently holds true in most
			// every situation. Later, this assumption may not hold as true.
			child, _ := context.WithTimeout(req.ctx, providerRequestTimeout)
			providers := bs.network.FindProvidersAsync(child, keys[0], maxProvidersPerRequest)
			err := bs.sendWantlistToPeers(req.ctx, providers)
			if err != nil {
				log.Debugf("error sending wantlist: %s", err)
			}
		case <-parent.Done():
			return
		}
	}
}

func (bs *bitswap) rebroadcastWorker(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	broadcastSignal := time.After(rebroadcastDelay.Get())

	for {
		select {
		case <-time.Tick(10 * time.Second):
			n := bs.wantlist.Len()
			if n > 0 {
				log.Debug(n, inflect.FromNumber("keys", n), "in bitswap wantlist")
			}
		case <-broadcastSignal: // resend unfulfilled wantlist keys
			entries := bs.wantlist.Entries()
			if len(entries) > 0 {
				bs.sendWantlistToProviders(ctx, entries)
			}
			broadcastSignal = time.After(rebroadcastDelay.Get())
		case <-parent.Done():
			return
		}
	}
}
