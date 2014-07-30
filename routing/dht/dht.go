package dht

import (
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"
	"code.google.com/p/goprotobuf/proto"
	"sync"
)

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	routes RoutingTable

	network *swarm.Swarm

	// map of channels waiting for reply messages
	listeners  map[uint64]chan *swarm.Message
	listenLock sync.RWMutex

	// Signal to shutdown dht
	shutdown chan struct{}
}

// Read in all messages from swarm and handle them appropriately
// NOTE: this function is just a quick sketch
func (dht *IpfsDHT) handleMessages() {
	for {
		select {
		case mes := <-dht.network.Chan.Incoming:
			pmes := new(DHTMessage)
			err := proto.Unmarshal(mes.Data, pmes)
			if err != nil {
				u.PErr("Failed to decode protobuf message: %s", err)
				continue
			}

			// Note: not sure if this is the correct place for this
			dht.listenLock.RLock()
			ch, ok := dht.listeners[pmes.GetId()]
			dht.listenLock.RUnlock()
			if ok {
				ch <- mes
			}
			//

			// Do something else with the messages?
			switch pmes.GetType() {
			case DHTMessage_ADD_PROVIDER:
			case DHTMessage_FIND_NODE:
			case DHTMessage_GET_PROVIDERS:
			case DHTMessage_GET_VALUE:
			case DHTMessage_PING:
			case DHTMessage_PUT_VALUE:
			}

		case <-dht.shutdown:
			return
		}
	}
}

// Register a handler for a specific message ID, used for getting replies
// to certain messages (i.e. response to a GET_VALUE message)
func (dht *IpfsDHT) ListenFor(mesid uint64) <-chan *swarm.Message {
	lchan := make(chan *swarm.Message)
	dht.listenLock.Lock()
	dht.listeners[mesid] = lchan
	dht.listenLock.Unlock()
	return lchan
}
