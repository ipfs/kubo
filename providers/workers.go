package providers

import (
	"context"
	"sync"

	process "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	procctx "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess/context"
	peer "gx/ipfs/QmVf8hTAsLLFtn4WPCRNdnaF2Eag2qTBS6uR8AiHPZARXy/go-libp2p-peer"
	pstore "gx/ipfs/QmZhsmorLpD9kmQ4ynbAu4vbKv2goMUnXazwGA4gnWHDjB/go-libp2p-peerstore"
	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
	logging "gx/ipfs/Qmbi1CTJsbnBZjCEgc2otwu8cUFPsGpzWXG7edVCLZ7Gvk/go-log"
)

func (p *providers) startWorkers(ctx context.Context, px process.Process) {
	if p.host != nil {
		// Start up a worker to handle block requests this node is making
		px.Go(func(px process.Process) {
			p.providerQueryManager(ctx)
		})
	}

	// Start up a worker to manage sending out provides messages
	px.Go(func(px process.Process) {
		p.provideCollector(ctx)
	})

	// Spawn up multiple workers to handle incoming blocks
	// consider increasing number if providing blocks bottlenecks
	// file transfers
	px.Go(p.provideWorker)
}

func (p *providers) provideWorker(px process.Process) {

	limit := make(chan struct{}, provideWorkerMax)

	limitedGoProvide := func(k *cid.Cid, wid int) {
		defer func() {
			// replace token when done
			<-limit
		}()
		ev := logging.LoggableMap{"ID": wid}

		ctx := procctx.OnClosingContext(px) // derive ctx from px
		defer log.EventBegin(ctx, "Bitswap.ProvideWorker.Work", ev, k).Done()

		ctx, cancel := context.WithTimeout(ctx, provideTimeout) // timeout ctx
		defer cancel()

		if err := p.routing.Provide(ctx, k, true); err != nil {
			log.Warning(err)
		}
	}

	// worker spawner, reads from bs.provideKeys until it closes, spawning a
	// _ratelimited_ number of workers to handle each key.
	for wid := 2; ; wid++ {
		ev := logging.LoggableMap{"ID": 1}
		log.Event(procctx.OnClosingContext(px), "Bitswap.ProvideWorker.Loop", ev)

		select {
		case <-px.Closing():
			return
		case k, ok := <-p.provideKeys:
			if !ok {
				log.Debug("provideKeys channel closed")
				return
			}
			select {
			case <-px.Closing():
				return
			case limit <- struct{}{}:
				go limitedGoProvide(k, wid)
			}
		}
	}
}

func (p *providers) provideCollector(ctx context.Context) {
	defer close(p.provideKeys)
	var toProvide []*cid.Cid
	var nextKey *cid.Cid
	var keysOut chan *cid.Cid

	for {
		select {
		case blkey, ok := <-p.newBlocks:
			if !ok {
				log.Debug("newBlocks channel closed")
				return
			}

			if keysOut == nil {
				nextKey = blkey
				keysOut = p.provideKeys
			} else {
				toProvide = append(toProvide, blkey)
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

func (p *providers) providerQueryManager(ctx context.Context) {
	var activeLk sync.Mutex
	kset := cid.NewSet()

	for {
		select {
		case e := <-p.findKeys:
			select { // make sure its not already cancelled
			case <-e.Ctx.Done():
				continue
			default:
			}

			activeLk.Lock()
			if kset.Has(e.Cid) {
				activeLk.Unlock()
				continue
			}
			kset.Add(e.Cid)
			activeLk.Unlock()

			go func(e *blockRequest) {
				child, cancel := context.WithTimeout(e.Ctx, providerRequestTimeout)
				defer cancel()
				providers := p.FindProvidersAsync(child, e.Cid, MaxProvidersPerRequest)
				wg := &sync.WaitGroup{}
				for pr := range providers {
					wg.Add(1)
					go func(pi peer.ID) {
						defer wg.Done()
						err := p.host.Connect(ctx, pstore.PeerInfo{ID: pi})
						if err != nil {
							log.Debug("failed to connect to provider %s: %s", p, err)
						}
					}(pr)
				}
				wg.Wait()
				activeLk.Lock()
				kset.Remove(e.Cid)
				activeLk.Unlock()
			}(e)

		case <-ctx.Done():
			return
		}
	}
}
