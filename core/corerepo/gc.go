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
	rmed, err := gc.GC(ctx, n.Blockstore, n.Pinning)
	if err != nil {
		return err
	}

	for {
		select {
		case _, ok := <-rmed:
			if !ok {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

}

func GarbageCollectAsync(n *core.IpfsNode, ctx context.Context) (<-chan *KeyRemoved, error) {
	rmed, err := gc.GC(ctx, n.Blockstore, n.Pinning)
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
	}()
	return out, nil
}
