package corerepo

import (
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"

	logging "github.com/ipfs/go-ipfs/vendor/go-log-v1.0.0"
)

var log = logging.Logger("corerepo")

type KeyRemoved struct {
	Key key.Key
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
						log.Debugf("Error removing key from blockstore: %s", err)
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
