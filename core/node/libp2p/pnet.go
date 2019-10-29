package libp2p

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	host "github.com/libp2p/go-libp2p-core/host"
	pnet "github.com/libp2p/go-libp2p-pnet"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/repo"
)

type PNetFingerprint []byte

func PNet(repo repo.Repo) (opts Libp2pOpts, fp PNetFingerprint, err error) {
	swarmkey, err := repo.SwarmKey()
	if err != nil || swarmkey == nil {
		return opts, nil, err
	}

	protec, err := pnet.NewProtector(bytes.NewReader(swarmkey))
	if err != nil {
		return opts, nil, fmt.Errorf("failed to configure private network: %s", err)
	}
	fp = protec.Fingerprint()

	opts.Opts = append(opts.Opts, libp2p.PrivateNetwork(protec))
	return opts, fp, nil
}

func PNetChecker(repo repo.Repo, ph host.Host, lc fx.Lifecycle) error {
	// TODO: better check?
	swarmkey, err := repo.SwarmKey()
	if err != nil || swarmkey == nil {
		return err
	}

	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				t := time.NewTicker(30 * time.Second)
				defer t.Stop()

				<-t.C // swallow one tick
				for {
					select {
					case <-t.C:
						if len(ph.Network().Peers()) == 0 {
							log.Warn("We are in private network and have no peers.")
							log.Warn("This might be configuration mistake.")
						}
					case <-done:
						return
					}
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			close(done)
			return nil
		},
	})
	return nil
}
