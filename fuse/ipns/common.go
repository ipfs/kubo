package ipns

import (
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	nsys "github.com/ipfs/go-ipfs/namesys"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	path "github.com/ipfs/go-ipfs/path"
	ft "github.com/ipfs/go-ipfs/unixfs"
)

// InitializeKeyspace sets the ipns record for the given key to
// point to an empty directory.
func InitializeKeyspace(n *core.IpfsNode, key ci.PrivKey) error {
	emptyDir := &dag.Node{Data: ft.FolderPBData()}
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
	if err := pub.Publish(ctx, key, path.FromKey(nodek)); err != nil {
		return err
	}

	return nil
}
