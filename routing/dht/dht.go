package dht

import (
	"sync"

	peer	"github.com/jbenet/go-ipfs/peer"
	swarm	"github.com/jbenet/go-ipfs/swarm"
	u		"github.com/jbenet/go-ipfs/util"

	ds "github.com/jbenet/datastore.go"

	"code.google.com/p/goprotobuf/proto"
)

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js

// IpfsDHT is an implementation of Kademlia with Coral and S/Kademlia modifications.
// It is used to implement the base IpfsRouting module.
type IpfsDHT struct {
	routes RoutingTable

	network *swarm.Swarm

	// Local peer (yourself)
	self *peer.Peer

	// Local data
	datastore ds.Datastore

	// map of channels waiting for reply messages
	listeners  map[uint64]chan *swarm.Message
	listenLock sync.RWMutex

	// Signal to shutdown dht
	shutdown chan struct{}
}

func NewDHT(p *peer.Peer) *IpfsDHT {
	dht := new(IpfsDHT)
	dht.self = p
	dht.network = swarm.NewSwarm(p)
	dht.listeners = make(map[uint64]chan *swarm.Message)
	dht.shutdown = make(chan struct{})
	return dht
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
				dht.handlePutValue(mes.Peer, pmes)
			case DHTMessage_FIND_NODE:
				dht.handleFindNode(mes.Peer, pmes)
			case DHTMessage_ADD_PROVIDER:
			case DHTMessage_GET_PROVIDERS:
			case DHTMessage_PING:
				dht.handleFindNode(mes.Peer, pmes)
			}

		case <-dht.shutdown:
			return
		}
	}
}

func (dht *IpfsDHT) handleGetValue(p *peer.Peer, pmes *DHTMessage) {
	dskey := ds.NewKey(pmes.GetKey())
	i_val, err := dht.datastore.Get(dskey)
	if err == nil {
		isResponse := true
		resp := new(DHTMessage)
		resp.Response = &isResponse
		resp.Id = pmes.Id
		resp.Key = pmes.Key

		val := i_val.([]byte)
		resp.Value = val

		mes := new(swarm.Message)
		mes.Peer = p
		mes.Data = []byte(resp.String())
	} else if err == ds.ErrNotFound {
		// Find closest node(s) to desired key and reply with that info
		// TODO: this will need some other metadata in the protobuf message
		//			to signal to the querying node that the data its receiving
		//			is actually a list of other nodes
	}
}

// Store a value in this nodes local storage
func (dht *IpfsDHT) handlePutValue(p *peer.Peer, pmes *DHTMessage) {
	dskey := ds.NewKey(pmes.GetKey())
	err := dht.datastore.Put(dskey, pmes.GetValue())
	if err != nil {
		// For now, just panic, handle this better later maybe
		panic(err)
	}
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

func (dht *IpfsDHT) Unlisten(mesid uint64) {
	dht.listenLock.Lock()
	ch, ok := dht.listeners[mesid]
	if ok {
		delete(dht.listeners, mesid)
	}
	dht.listenLock.Unlock()
	close(ch)
}


// Stop all communications from this node and shut down
func (dht *IpfsDHT) Halt() {
	dht.shutdown <- struct{}{}
	dht.network.Close()
}
