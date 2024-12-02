package ipns

import (
	"context"

	ft "github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/namesys"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core"
	ci "github.com/libp2p/go-libp2p/core/crypto"
)

// InitializeKeyspace sets the ipns record for the given key to
// point to an empty directory.
func InitializeKeyspace(n *core.IpfsNode, key ci.PrivKey) error {
	ctx, cancel := context.WithCancel(n.Context())
	defer cancel()

	emptyDir := ft.EmptyDirNode()

	err := n.Pinning.Pin(ctx, emptyDir, false, "")
	if err != nil {
		return err
	}

	err = n.Pinning.Flush(ctx)
	if err != nil {
		return err
	}

	pub := namesys.NewIPNSPublisher(n.Routing, n.Repo.Datastore())

	return pub.Publish(ctx, key, path.FromCid(emptyDir.Cid()))
}
