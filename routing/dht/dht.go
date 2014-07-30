package dht

import (
	swarm "github.com/jbenet/go-ipfs/swarm"
	"sync"
)

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	routes RoutingTable

	network *swarm.Swarm

	listeners  map[uint64]chan swarm.Message
	listenLock sync.RWMutex
}

// Read in all messages from swarm and handle them appropriately
// NOTE: this function is just a quick sketch
func (dht *IpfsDHT) handleMessages() {
	for mes := range dht.network.Chan.Incoming {
		for {
			select {
			case mes := <-dht.network.Chan.Incoming:
				// Unmarshal message
				dht.listenLock.RLock()
				ch, ok := dht.listeners[id]
				dht.listenLock.RUnlock()
				if ok {
					// Send message to waiting goroutine
					ch <- mes
				}

				//case closeChan: or something
			}
		}
	}
}

// Register a handler for a specific message ID, used for getting replies
// to certain messages (i.e. response to a GET_VALUE message)
func (dht *IpfsDHT) ListenFor(mesid uint64) <-chan swarm.Message {
	lchan := make(chan swarm.Message)
	dht.listenLock.Lock()
	dht.listeners[mesid] = lchan
	dht.listenLock.Unlock()
	return lchan
}
