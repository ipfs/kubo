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
func Handshake3Msg(localPeer peer.Peer, remoteAddr ma.Multiaddr) *pb.Handshake3 {
	var msg pb.Handshake3
	// don't need publicKey after secure channel.
	// msg.PublicKey = localPeer.PubKey().Bytes()

	// local listen addresses
	addrs := localPeer.Addresses()
	msg.ListenAddrs = make([][]byte, len(addrs))
	for i, a := range addrs {
		msg.ListenAddrs[i] = a.Bytes()
	}

	// observed remote address
	msg.ObservedAddr = remoteAddr.Bytes()

	// services
	// srv := localPeer.Services()
	// msg.Services = make([]mux.ProtocolID, len(srv))
	// for i, pid := range srv {
	// 	msg.Services[i] = pid
	// }

	return &msg
}

// Handshake3Update updates local knowledge with the information in the
// handshake3 msg we received from remote client.
func Handshake3Update(lpeer, rpeer peer.Peer, msg *pb.Handshake3) (*Handshake3Result, error) {
	res := &Handshake3Result{}

	// our observed address
	observedAddr, err := ma.NewMultiaddrBytes(msg.GetObservedAddr())
	if err != nil {
		return res, err
	}
	if lpeer.AddAddress(observedAddr) {
		log.Infof("(nat) added new local, remote-observed address: %s", observedAddr)
	}
	res.LocalObservedAddress = observedAddr

	// remote's reported addresses
	for _, a := range msg.GetListenAddrs() {
		addr, err := ma.NewMultiaddrBytes(a)
		if err != nil {
			err = fmt.Errorf("remote peer address not a multiaddr: %s", err)
			log.Errorf("Handshake3 error %s", err)
			return res, err
		}
		rpeer.AddAddress(addr)
		res.RemoteListenAddresses = append(res.RemoteListenAddresses, addr)
	}

	return res, nil
}

// Handshake3Result collects the knowledge gained in Handshake3.
type Handshake3Result struct {

	// The addresses reported by the remote client
	RemoteListenAddresses []ma.Multiaddr

	// The address of the remote client we observed in this connection
	RemoteObservedAddress ma.Multiaddr

	// The address the remote client observed from this connection
	LocalObservedAddress ma.Multiaddr
}
