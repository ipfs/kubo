package dht

import (
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"

	inet "gx/ipfs/QmRW2xiYTpDLWTHb822ZYbPBoh3dGLJwaXLGS9tnPyWZpq/go-libp2p/p2p/net"
)

// netNotifiee defines methods to be used with the IpfsDHT
type netNotifiee IpfsDHT

func (nn *netNotifiee) DHT() *IpfsDHT {
	return (*IpfsDHT)(nn)
}

func (nn *netNotifiee) Connected(n inet.Network, v inet.Conn) {
	dht := nn.DHT()
	select {
	case <-dht.Process().Closing():
		return
	default:
	}
	dht.Update(dht.Context(), v.RemotePeer())
}

func (nn *netNotifiee) Disconnected(n inet.Network, v inet.Conn) {
	dht := nn.DHT()
	select {
	case <-dht.Process().Closing():
		return
	default:
	}
	dht.routingTable.Remove(v.RemotePeer())
}

func (nn *netNotifiee) OpenedStream(n inet.Network, v inet.Stream) {}
func (nn *netNotifiee) ClosedStream(n inet.Network, v inet.Stream) {}
func (nn *netNotifiee) Listen(n inet.Network, a ma.Multiaddr)      {}
func (nn *netNotifiee) ListenClose(n inet.Network, a ma.Multiaddr) {}
