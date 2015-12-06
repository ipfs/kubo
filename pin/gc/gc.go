package gc

import (
	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	dag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	logging "github.com/ipfs/go-ipfs/vendor/QmQg1J6vikuXF9oDvm4wpdeAUvvkVEKW1EYDw9HhTMnP2b/go-log"
)

var log = logging.Logger("gc")

// GC performs a mark and sweep garbage collection of the blocks in the blockstore
// first, it creates a 'marked' set and adds to it the following:
// - all recursively pinned blocks, plus all of their descendants (recursively)
// - all directly pinned blocks
// - all blocks utilized internally by the pinner
//
// The routine then iterates over every block in the blockstore and
// deletes any block that is not found in the marked set.
func GC(ctx context.Context, bs bstore.GCBlockstore, pn pin.Pinner) (<-chan key.Key, error) {
	unlock := bs.GCLock()

	bsrv := bserv.New(bs, offline.Exchange(bs))
	ds := dag.NewDAGService(bsrv)

	gcs, err := ColoredSet(pn, ds)
	if err != nil {
		return nil, err
	}

	keychan, err := bs.AllKeysChan(ctx)
	if err != nil {
		return nil, err
	}

	output := make(chan key.Key)
	go func() {
		defer close(output)
		defer unlock()
		for {
			select {
			case k, ok := <-keychan:
				if !ok {
					return
				}
				if !gcs.Has(k) {
					err := bs.DeleteBlock(k)
					if err != nil {
						log.Debugf("Error removing key from blockstore: %s", err)
						return
					}
					select {
					case output <- k:
					case <-ctx.Done():
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return output, nil
}

func Descendants(ds dag.DAGService, set key.KeySet, roots []key.Key) error {
	for _, k := range roots {
		set.Add(k)
		nd, err := ds.Get(context.Background(), k)
		if err != nil {
			return err
		}

		// EnumerateChildren recursively walks the dag and adds the keys to the given set
		err = dag.EnumerateChildren(context.Background(), ds, nd, set)
		if err != nil {
			return err
		}
	}

	return nil
}

func ColoredSet(pn pin.Pinner, ds dag.DAGService) (key.KeySet, error) {
	// KeySet currently implemented in memory, in the future, may be bloom filter or
	// disk backed to conserve memory.
	gcs := key.NewKeySet()
	err := Descendants(ds, gcs, pn.RecursiveKeys())
	if err != nil {
		return nil, err
	}

	for _, k := range pn.DirectKeys() {
		gcs.Add(k)
	}

	err = Descendants(ds, gcs, pn.InternalPins())
	if err != nil {
		return nil, err
	}

	return gcs, nil
}
