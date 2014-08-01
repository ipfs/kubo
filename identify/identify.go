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

	// temporary:
	// put your own id in a 16byte buffer and send that over to
	// the peer as your ID, then wait for them to send their ID.
	// Once that trade is finished, the handshake is complete and
	// both sides should 'trust' each other

	out <- self.ID
	resp := <-in
	remote.ID = peer.ID(resp)
	u.DOut("Got node id: %s", string(remote.ID))

	return nil
}
