package libp2p

import (
	"time"

	version "github.com/ipfs/go-ipfs"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"go.uber.org/fx"
)

var log = logging.Logger("p2pnode")

type Libp2pOpts struct {
	fx.Out

	Opts []libp2p.Option `group:"libp2p"`
}

// Misc options

var UserAgent = simpleOpt(libp2p.UserAgent(version.UserAgent))

func ConnectionManager(low, high int, grace time.Duration) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		cm := connmgr.NewConnManager(low, high, grace)
		opts.Opts = append(opts.Opts, libp2p.ConnectionManager(cm))
		return
	}
}

func PstoreAddSelfKeys(id peer.ID, sk crypto.PrivKey, ps peerstore.Peerstore) error {
	if err := ps.AddPubKey(id, sk.GetPublic()); err != nil {
		return err
	}

	return ps.AddPrivKey(id, sk)
}

func simpleOpt(opt libp2p.Option) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		opts.Opts = append(opts.Opts, opt)
		return
	}
}
