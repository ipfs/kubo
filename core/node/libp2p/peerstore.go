package libp2p

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	"go.uber.org/fx"
)

func Peerstore(lc fx.Lifecycle) (peerstore.Peerstore, error) {
	pstore, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return pstore.Close()
		},
	})

	return pstore, nil
}
