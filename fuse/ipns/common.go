package ipns

import (
	"context"

	"github.com/ipfs/go-ipfs/core"
	nsys "github.com/ipfs/go-ipfs/namesys"
	ft "gx/ipfs/QmWdTRLi3H7ZJQ8s7NYo8oitz5JHEEPKLn1QPMsJVWg2Ew/go-unixfs"
	ci "gx/ipfs/Qme1knMqwt1hKZbc1BmQFmnm9f36nyQGwXxPGVpVJ9rMK5/go-libp2p-crypto"
	path "gx/ipfs/Qme34dT9spiPgunbueNtziRX4SvfLHDFZQvmTBVK8p4qNT/go-path"
)

// InitializeKeyspace sets the ipns record for the given key to
// point to an empty directory.
func InitializeKeyspace(n *core.IpfsNode, key ci.PrivKey) error {
	ctx, cancel := context.WithCancel(n.Context())
	defer cancel()

	emptyDir := ft.EmptyDirNode()

	err := n.Pinning.Pin(ctx, emptyDir, false)
	if err != nil {
		return err
	}

	err = n.Pinning.Flush()
	if err != nil {
		return err
	}

	pub := nsys.NewIpnsPublisher(n.Routing, n.Repo.Datastore())

	return pub.Publish(ctx, key, path.FromCid(emptyDir.Cid()))
}
