package handshake

import (
	"fmt"

	pb "github.com/jbenet/go-ipfs/net/handshake/pb"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

var log = u.Logger("handshake")

// Handshake3Msg constructs a Handshake3 msg.
func Handshake3Msg(localPeer peer.Peer) *pb.Handshake3 {
	var msg pb.Handshake3
	// don't need publicKey after secure channel.
	// msg.PublicKey = localPeer.PubKey().Bytes()

	// addresses
	addrs := localPeer.Addresses()
	msg.ListenAddrs = make([][]byte, len(addrs))
	for i, a := range addrs {
		msg.ListenAddrs[i] = a.Bytes()
	}

	// services
	// srv := localPeer.Services()
	// msg.Services = make([]mux.ProtocolID, len(srv))
	// for i, pid := range srv {
	// 	msg.Services[i] = pid
	// }

	return &msg
}

// Handshake3UpdatePeer updates a remote peer with the information in the
// handshake3 msg we received from them.
func Handshake3UpdatePeer(remotePeer peer.Peer, msg *pb.Handshake3) error {

	// addresses
	for _, a := range msg.GetListenAddrs() {
		addr, err := ma.NewMultiaddrBytes(a)
		if err != nil {
			err = fmt.Errorf("remote peer address not a multiaddr: %s", err)
			log.Error("Handshake3: error %s", err)
			return err
		}
		remotePeer.AddAddress(addr)
	}

	return nil
}
