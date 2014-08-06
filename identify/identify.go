// The identify package handles how peers identify with eachother upon
// connection to the network
package identify

import (
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// Perform initial communication with this peer to share node ID's and
// initiate communication
func Handshake(self, remote *peer.Peer, in, out chan []byte) error {
	// TODO: make this more... secure.
	out <- self.ID
	resp := <-in
	remote.ID = peer.ID(resp)
	u.DOut("identify: Got node id: %s", remote.ID.Pretty())

	return nil
}
