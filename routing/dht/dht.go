package dht

import (
	"sync"

	peer "github.com/jbenet/go-ipfs/peer"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"
	"code.google.com/p/goprotobuf/proto"
)

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	routes RoutingTable

	network *swarm.Swarm

	// local data (TEMPORARY: until we formalize data storage with datastore)
	data map[string][]byte

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
			if pmes.GetResponse() {
				dht.listenLock.RLock()
				ch, ok := dht.listeners[pmes.GetId()]
				dht.listenLock.RUnlock()
				if ok {
					ch <- mes
				}

				// this is expected behaviour during a timeout
				u.DOut("Received response with nobody listening...")
				continue
			}
			//

			switch pmes.GetType() {
			case DHTMessage_GET_VALUE:
				dht.handleGetValue(mes.Peer, pmes)
			case DHTMessage_PUT_VALUE:
			case DHTMessage_FIND_NODE:
			case DHTMessage_ADD_PROVIDER:
			case DHTMessage_GET_PROVIDERS:
			case DHTMessage_PING:
			}

		case <-dht.shutdown:
			return
		}
	}
}

func (dht *IpfsDHT) handleGetValue(p *peer.Peer, pmes *DHTMessage) {
	val, found := dht.data[pmes.GetKey()]
	if found {
		isResponse := true
		resp := new(DHTMessage)
		resp.Response = &isResponse
		resp.Id = pmes.Id
		resp.Key = pmes.Key
		resp.Value = val
	} else {
		// Find closest node(s) to desired key and reply with that info
		// TODO: this will need some other metadata in the protobuf message
		//			to signal to the querying node that the data its receiving
		//			is actually a list of other nodes
	}
}

func (dht *IpfsDHT) handlePutValue(p *peer.Peer, pmes *DHTMessage) {
	panic("Not implemented.")
}

func (dht *IpfsDHT) handleFindNode(p *peer.Peer, pmes *DHTMessage) {
	panic("Not implemented.")
}

func (dht *IpfsDHT) handlePing(p *peer.Peer, pmes *DHTMessage) {
	isResponse := true
	resp := new(DHTMessage)
	resp.Id = pmes.Id
	resp.Response = &isResponse

	mes := new(swarm.Message)
	mes.Peer = p
	mes.Data = []byte(resp.String())
	dht.network.Chan.Outgoing <- mes
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

// Stop all communications from this node and shut down
func (dht *IpfsDHT) Halt() {
	dht.shutdown <- struct{}{}
	dht.network.Close()
}
