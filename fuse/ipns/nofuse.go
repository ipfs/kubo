// +build nofuse

package ipns

import (
	"errors"

	"github.com/jbenet/go-ipfs/core"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
)

// InitializeKeyspace sets the ipns record for the given key to
// point to an empty directory.
func InitializeKeyspace(n *core.IpfsNode, key ci.PrivKey) error {
	// emptyDir := &mdag.Node{Data: ft.FolderPBData()}
	// nodek, err := n.DAG.Add(emptyDir)
	// if err != nil {
	// 	return err
	// }

	// err = n.Pinning.Pin(emptyDir, false)
	// if err != nil {
	// 	return err
	// }

	// err = n.Pinning.Flush()
	// if err != nil {
	// 	return err
	// }

	// pub := nsys.NewRoutingPublisher(n.Routing)
	// err = pub.Publish(n.Context(), key, nodek)
	// if err != nil {
	// 	return err
	// }

	return errors.New("how is this fuse related?")
}
