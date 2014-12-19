package net

import (
	handshake "github.com/jbenet/go-ipfs/net/handshake"
	pb "github.com/jbenet/go-ipfs/net/handshake/pb"

	ggio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/io"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// IDService is a structure that implements ProtocolIdentify.
// It is a trivial service that gives the other peer some
// useful information about the local peer. A sort of hello.
//
// The IDService sends:
//  * Our IPFS Protocol Version
//  * Our IPFS Agent Version
//  * Our public Listen Addresses
type IDService struct {
	Network Network
}

func NewIDService(n Network) *IDService {
	s := &IDService{Network: n}
	n.SetHandler(ProtocolIdentify, s.RequestHandler)
	return s
}

func (ids *IDService) RequestHandler(s Stream) {
	defer s.Close()
	c := s.Conn()

	w := ggio.NewDelimitedWriter(s)
	mes := pb.Handshake3{}
	ids.populateMessage(&mes, s.Conn())
	w.WriteMsg(&mes)

	log.Debugf("%s sent message to %s %s", ProtocolIdentify,
		c.RemotePeer(), c.RemoteMultiaddr())
}

func (ids *IDService) ResponseHandler(s Stream) {
	defer s.Close()
	c := s.Conn()

	r := ggio.NewDelimitedReader(s, 2048)
	mes := pb.Handshake3{}
	if err := r.ReadMsg(&mes); err != nil {
		log.Errorf("%s error receiving message from %s %s", ProtocolIdentify,
			c.RemotePeer(), c.RemoteMultiaddr())
		return
	}
	ids.consumeMessage(&mes, c)

	log.Debugf("%s received message from %s %s", ProtocolIdentify,
		c.RemotePeer(), c.RemoteMultiaddr())
}

func (ids *IDService) populateMessage(mes *pb.Handshake3, c Conn) {

	// set protocols this node is currently handling
	protos := ids.Network.Protocols()
	mes.Protocols = make([]string, len(protos))
	for i, p := range protos {
		mes.Protocols[i] = string(p)
	}

	// observed address so other side is informed of their
	// "public" address, at least in relation to us.
	mes.ObservedAddr = c.RemoteMultiaddr().Bytes()

	// set listen addrs
	laddrs := ids.Network.ListenAddresses()
	mes.ListenAddrs = make([][]byte, len(laddrs))
	for i, addr := range laddrs {
		mes.ListenAddrs[i] = addr.Bytes()
	}

	// set protocol versions
	mes.H1 = handshake.NewHandshake1("", "")
}

func (ids *IDService) consumeMessage(mes *pb.Handshake3, c Conn) {
	p := c.RemotePeer()

	// mes.Protocols
	// mes.ObservedAddr

	// mes.ListenAddrs
	laddrs := mes.GetListenAddrs()
	lmaddrs := make([]ma.Multiaddr, 0, len(laddrs))
	for _, addr := range laddrs {
		maddr, err := ma.NewMultiaddrBytes(addr)
		if err != nil {
			log.Errorf("%s failed to parse multiaddr from %s %s", ProtocolIdentify, p,
				c.RemoteMultiaddr())
			continue
		}
		lmaddrs = append(lmaddrs, maddr)
	}

	// update our peerstore with the addresses.
	ids.Network.Peerstore().AddAddresses(p, lmaddrs)

	// get protocol versions
	pv := *mes.H1.ProtocolVersion
	av := *mes.H1.AgentVersion
	ids.Network.Peerstore().Put(p, "ProtocolVersion", pv)
	ids.Network.Peerstore().Put(p, "AgentVersion", av)
}
