package main

import (
	"context"
	"fmt"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
)

type providerBlockstore struct {
	backend bstore.Blockstore
	keys    map[cid.Cid]bool
}

func NewProviderBlockstore(bs bstore.Blockstore) *providerBlockstore {
	pds := &providerBlockstore{
		backend: bs,
		keys:    make(map[cid.Cid]bool),
	}
	return pds
}

const (
	indexProviderPollInterval = time.Minute * 2
)

func (pds *providerBlockstore) startBlockstoreProvider(cctx idxProviderContext, node idxProviderNode) {
	errCh := make(chan error)

	go pds.providerOnChange(cctx, node, errCh)
	go func() {
		for {
			select {
			case err, isOpen := <-errCh:
				providerlog.Debugf("got error in errCh")
				if !isOpen {
					providerlog.Debugf("errCh not open")
					return
				}
				providerlog.Errorf("%v", err)
			case <-cctx.Context().Done():
				providerlog.Debugf("Context done")
				return
			}
		}
	}()
}

func (pds *providerBlockstore) providerOnChange(cctx idxProviderContext, node idxProviderNode, errCh chan<- error) {
	defer close(errCh)

	var tmo *time.Timer
	defer func() {
		if tmo != nil {
			tmo.Stop()
		}
	}()

	for {
		// polling sleep
		if tmo == nil {
			tmo = time.NewTimer(indexProviderPollInterval)
		} else {
			tmo.Reset(indexProviderPollInterval)
		}
		select {
		case <-cctx.Context().Done():
			providerlog.Debugf("context done")
			return
		case <-tmo.C:
			providerlog.Debugf("tick")
		}

		providerlog.Errorf("indexer provider is awake")

		// advertise to indexer nodes
		providerlog.Debugf("advertising to indexer")
		var cids []cid.Cid
		var cnt = 0
		for k := range pds.keys {
			_, c, _ := cid.CidFromBytes(k.Bytes())
			cnt++
			cids = append(cids, c)
			if cnt >= maxCidsPerAdv {
				if err := doAdvertisement(cctx.Context(), node, cids); err != nil {
					providerlog.Debugf("error from doAdvertisement")
					errCh <- fmt.Errorf("error advertising latest to indexer (%v)", err)
				}
				cids = cids[:0]
				cnt = 0
			}
		}
		if err := doAdvertisement(cctx.Context(), node, cids); err != nil {
			providerlog.Debugf("error from doAdvertisement")
			errCh <- fmt.Errorf("error advertising latest to indexer (%v)", err)
		}
		// reset pds.keys
		pds.keys = make(map[cid.Cid]bool)
	}
}

func (bs *providerBlockstore) HashOnRead(enabled bool) {
	bs.backend.HashOnRead(enabled)
}

func (bs *providerBlockstore) Get(ctx context.Context, k cid.Cid) (blocks.Block, error) {
	return bs.backend.Get(ctx, k)
}

func (bs *providerBlockstore) Put(ctx context.Context, block blocks.Block) error {
	bs.keys[block.Cid()] = true
	return bs.backend.Put(ctx, block)
}

func (bs *providerBlockstore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	for _, b := range blocks {
		bs.keys[b.Cid()] = true
	}
	return bs.backend.PutMany(ctx, blocks)
}

func (bs *providerBlockstore) Has(ctx context.Context, k cid.Cid) (bool, error) {
	return bs.backend.Has(ctx, k)
}

func (bs *providerBlockstore) GetSize(ctx context.Context, k cid.Cid) (int, error) {
	return bs.backend.GetSize(ctx, k)
}

func (bs *providerBlockstore) DeleteBlock(ctx context.Context, k cid.Cid) error {
	if bs.keys[k] {
		delete(bs.keys, k)
	}
	return bs.backend.DeleteBlock(ctx, k)
}

func (bs *providerBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return bs.backend.AllKeysChan(ctx)
}
