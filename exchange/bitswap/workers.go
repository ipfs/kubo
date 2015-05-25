package bitswap

import (
	"os"
	"strconv"
	"time"

	process "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	u "github.com/ipfs/go-ipfs/util"
)

var TaskWorkerCount = 8

func init() {
	twc := os.Getenv("IPFS_BITSWAP_TASK_WORKERS")
	if twc != "" {
		n, err := strconv.Atoi(twc)
		if err != nil {
			log.Error(err)
			return
		}
		if n > 0 {
			TaskWorkerCount = n
		} else {
			log.Errorf("Invalid value of '%d' for IPFS_BITSWAP_TASK_WORKERS", n)
		}
	}
}

func (bs *Bitswap) startWorkers(px process.Process, ctx context.Context) {
	// Start up a worker to handle block requests this node is making
	px.Go(func(px process.Process) {
		bs.clientWorker(ctx)
	})

	// Start up workers to handle requests from other nodes for the data on this node
	for i := 0; i < TaskWorkerCount; i++ {
		px.Go(func(px process.Process) {
			bs.taskWorker(ctx)
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
	for i := 0; i < provideWorkers; i++ {
		px.Go(func(px process.Process) {
			bs.provideWorker(ctx)
		})
	}
}

func (bs *Bitswap) taskWorker(ctx context.Context) {
	defer log.Info("bitswap task worker shutting down...")
	for {
		select {
		case nextEnvelope := <-bs.engine.Outbox():
			select {
			case envelope, ok := <-nextEnvelope:
				if !ok {
					continue
				}

				bs.wm.SendBlock(ctx, envelope)
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (bs *Bitswap) provideWorker(ctx context.Context) {
	for {
		select {
		case k, ok := <-bs.provideKeys:
			if !ok {
				log.Debug("provideKeys channel closed")
				return
			}
			ctx, cancel := context.WithTimeout(ctx, provideTimeout)
			err := bs.network.Provide(ctx, k)
			if err != nil {
				log.Error(err)
			}
			cancel()
		case <-ctx.Done():
			return
		}
	}
}

func (bs *Bitswap) provideCollector(ctx context.Context) {
	defer close(bs.provideKeys)
	var toProvide []u.Key
	var nextKey u.Key
	var keysOut chan u.Key

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

// TODO: figure out clientWorkers purpose in life
func (bs *Bitswap) clientWorker(parent context.Context) {
	defer log.Info("bitswap client worker shutting down...")

	for {
		select {
		case req := <-bs.batchRequests:
			keys := req.keys
			if len(keys) == 0 {
				log.Warning("Received batch request for zero blocks")
				continue
			}

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
		select {
		case <-tick.C:
			n := bs.wm.wl.Len()
			if n > 0 {
				log.Debug(n, "keys in bitswap wantlist")
			}
		case <-broadcastSignal.C: // resend unfulfilled wantlist keys
			entries := bs.wm.wl.Entries()
			if len(entries) > 0 {
				bs.connectToProviders(ctx, entries)
			}
		case <-parent.Done():
			return
		}
	}
}
