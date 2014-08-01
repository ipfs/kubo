// The identify package handles how peers identify with eachother upon
// connection to the network
package identify

import (
	peer "github.com/jbenet/go-ipfs/peer"
	swarm "github.com/jbenet/go-ipfs/swarm"
)

// Perform initial communication with this peer to share node ID's and
// initiate communication
func Handshake(self *peer.Peer, conn *swarm.Conn) error {

	// temporary:
	// put your own id in a 16byte buffer and send that over to
	// the peer as your ID, then wait for them to send their ID.
	// Once that trade is finished, the handshake is complete and
	// both sides should 'trust' each other

	id := make([]byte, 16)
	copy(id, self.ID)

	conn.Outgoing.MsgChan <- id
	resp := <-conn.Incoming.MsgChan
	conn.Peer.ID = peer.ID(resp)

	return nil
}
