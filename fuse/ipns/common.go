package ipns

import (
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	"github.com/ipfs/go-ipfs/core"
	nsys "github.com/ipfs/go-ipfs/namesys"
	path "github.com/ipfs/go-ipfs/path"
	ft "github.com/ipfs/go-ipfs/unixfs"
	ci "gx/ipfs/QmVoi5es8D5fNHZDqoW6DgDAEPEV5hQp8GBz161vZXiwpQ/go-libp2p-crypto"
)

// InitializeKeyspace sets the ipns record for the given key to
// point to an empty directory.
func InitializeKeyspace(n *core.IpfsNode, key ci.PrivKey) error {
	emptyDir := ft.EmptyDirNode()
	nodek, err := n.DAG.Add(emptyDir)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(n.Context())
	defer cancel()

	err = n.Pinning.Pin(ctx, emptyDir, false)
	if err != nil {
		return err
	}

	err = n.Pinning.Flush()
	if err != nil {
		return err
	}

	pub := nsys.NewRoutingPublisher(n.Routing, n.Repo.Datastore())
	if err := pub.Publish(ctx, key, path.FromCid(nodek)); err != nil {
		return err
	}

	return nil
}
