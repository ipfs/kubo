package ipns

import (
	"context"

	"github.com/ipfs/go-ipfs/core"
	nsys "github.com/ipfs/go-ipfs/namesys"
	path "gx/ipfs/QmWZr1S1C28AePSC2zBVsU4WKQqe6AxbiFu9SeDtD7Xkdw/go-path"
	ft "gx/ipfs/QmZBYfq7U5FXirpJDvnsm3L8PncRNXnL7yziDR44jWqwUr/go-unixfs"
	ci "gx/ipfs/QmaZ1F8vQdJaQqA8hjAzH8a7Qnqqb2n2ZYeUbKMjMcxAZe/go-libp2p-crypto"
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
