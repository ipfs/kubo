package libp2p

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-ipfs/repo"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/pnet"
	"go.uber.org/fx"
	"golang.org/x/crypto/salsa20"
	"golang.org/x/crypto/sha3"
)

type PNetFingerprint []byte

func PNet(repo repo.Repo) (opts Libp2pOpts, fp PNetFingerprint, err error) {
	swarmkey, err := repo.SwarmKey()
	if err != nil || swarmkey == nil {
		return opts, nil, err
	}

	psk, err := pnet.DecodeV1PSK(bytes.NewReader(swarmkey))
	if err != nil {
		return opts, nil, fmt.Errorf("failed to configure private network: %s", err)
	}

	opts.Opts = append(opts.Opts, libp2p.PrivateNetwork(psk))

	return opts, pnetFingerprint(psk), nil
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

func pnetFingerprint(psk pnet.PSK) []byte {
	var pskArr [32]byte
	copy(pskArr[:], psk)

	enc := make([]byte, 64)
	zeros := make([]byte, 64)
	out := make([]byte, 16)

	// We encrypt data first so we don't feed PSK to hash function.
	// Salsa20 function is not reversible thus increasing our security margin.
	salsa20.XORKeyStream(enc, zeros, []byte("finprint"), &pskArr)

	// Then do Shake-128 hash to reduce its length.
	// This way if for some reason Shake is broken and Salsa20 preimage is possible,
	// attacker has only half of the bytes necessary to recreate psk.
	sha3.ShakeSum128(out, enc)

	return out
}
