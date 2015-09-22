package corerepo

import (
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
	gc "github.com/ipfs/go-ipfs/pin/gc"

	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("corerepo")

type KeyRemoved struct {
	Key key.Key
}

func GarbageCollect(n *core.IpfsNode, ctx context.Context) error {
	kr, err := GarbageCollectAsync(n, ctx)
	if err != nil {
		return err
	}

	// block until gc completes
	for _ = range kr {
	}

	return nil
}

func GarbageCollectAsync(n *core.IpfsNode, ctx context.Context) (<-chan *KeyRemoved, error) {
	// GC blocks from data blockstore
	// the gc.GC call will use the pinner to enumerate the colored set
	// and then call 'gc.RunGC' internally.
	rmed, err := gc.GC(ctx, n.DataBlocks, n.Pinning)
	if err != nil {
		return nil, err
	}

	// GC blocks from state blockstore
	ks := key.NewKeySet()
	for _, k := range n.Pinning.InternalPins() {
		ks.Add(k)
	}

	// for the nodestate blockstore, we manually enumerate the colored set
	// from the pinners internal pins. And then make a direct call to 'gc.RunGC' here
	internal, err := gc.RunGC(ctx, n.StateBlocks, ks)
	if err != nil {
		return nil, err
	}

	out := make(chan *KeyRemoved)
	go func() {
		defer close(out)
		for k := range rmed {
			select {
			case out <- &KeyRemoved{k}:
			case <-ctx.Done():
				return
			}
		}
		for k := range internal {
			select {
			case out <- &KeyRemoved{k}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
