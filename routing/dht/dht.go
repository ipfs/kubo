package dht

import (
	swarm "github.com/jbenet/go-ipfs/swarm"
)

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js


// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
  routes RoutingTable

  network *swarm.Swarm
}

func (dht *IpfsDHT) handleMessages() {
	for mes := range dht.network.Chan.Incoming {
		
	}
}
