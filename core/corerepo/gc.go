package corerepo

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"

	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("corerepo")

type KeyRemoved struct {
	Key u.Key
}

func GarbageCollect(n *core.IpfsNode, ctx context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // in case error occurs during operation
	keychan, err := n.Blockstore.AllKeysChan(ctx)
	if err != nil {
		return err
	}
	for k := range keychan { // rely on AllKeysChan to close chan
		if !n.Pinning.IsPinned(k) {
			err := n.Blockstore.DeleteBlock(k)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func GarbageCollectAsync(n *core.IpfsNode, ctx context.Context) (<-chan *KeyRemoved, error) {

	keychan, err := n.Blockstore.AllKeysChan(ctx)
	if err != nil {
		return nil, err
	}

	output := make(chan *KeyRemoved)
	go func() {
		defer close(output)
		for {
			select {
			case k, ok := <-keychan:
				if !ok {
					return
				}
				if !n.Pinning.IsPinned(k) {
					err := n.Blockstore.DeleteBlock(k)
					if err != nil {
						log.Errorf("Error removing key from blockstore: %s", err)
						continue
					}
					select {
					case output <- &KeyRemoved{k}:
					case <-ctx.Done():
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return output, nil
}
