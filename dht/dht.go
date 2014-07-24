package dht

import (
	"time"
	mh "github.com/jbenet/go-multihash"
	peer "github.com/jbenet/go-ipfs/peer"
	"errors"
	"net"
)

var NotFound = errors.New("Not Found")
var NotAvailable = errors.New("Not Available")
var TimeoutExceeded = errors.New("Timeout Exceeded")

// The IPFS DHT is an implementation of Kademlia with
// Coral and S/Kademlia modifications. It is used to
// implement the base IPFS Routing module.

// TODO. SEE https://github.com/jbenet/node-ipfs/blob/master/submodules/ipfs-dht/index.js
type DHT struct {
	//Network
	Network net.Conn

	// DHT Configuration Settings
	Config DHTConfig

	//Republish
	Republish *DHTRepublish
}

// TODO: not call this republish
type DHTRepublish struct {
	Strict []*DHTObject
	Sloppy []*DHTObject
}

type DHTObject struct {
	Key string
	Value *DHTValue
	LastPublished *time.Time
}

func (o *DHTObject) ShouldRepublish(interval time.Duration) bool {
	return (time.Now().Second() - o.LastPublished.Second()) > int(interval.Seconds())
}

// A struct representing a value in the DHT
type DHTValue struct {}

type DHTConfig struct {
	// Time to wait between republishing intervals
	RepublishInterval time.Duration

	// Multihash hash function
	HashType int
}

// Looks for a particular node
func (dht *DHT) FindNode(id *peer.ID /* and a callback? */) error {
	panic("Not implemented.")
}

func (dht *DHT) PingNode(id *peer.ID, timeout time.Duration) error {
	panic("Not implemented.")
}

// Retrieves a value for a given key
func (dht *DHT) GetValue(key string) *DHTValue {
	panic("Not implemented.")
}

// Stores a value for a given key
func (dht *DHT) SetValue(key string, value *DHTValue) error {
	panic("Not implemented.")
}

// GetSloppyValues finds (at least) a number of values for given key
func (dht *DHT) GetSloppyValues(key string, count int) ([]*DHTValue, error) {
	panic("Not implemented.")
}

func (dht *DHT) SetSloppyValue(key string, value *DHTValue) error {
	panic("Not implemented.")
}

func (dht *DHT) periodicRepublish() {
	tick := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-tick.C:
			for _,v := range dht.Republish.Strict {
				if v.ShouldRepublish(dht.Config.RepublishInterval) {
					dht.SetValue(v.Key, v.Value)
				}
			}

			for _,v := range dht.Republish.Sloppy {
				if v.ShouldRepublish(dht.Config.RepublishInterval) {
					dht.SetSloppyValue(v.Key, v.Value)
				}
			}
		}
	}
}

func (dht *DHT) handleMessage(message []byte) {

}

func (dht *DHT) coerceMultihash(hash mh.Multihash) {
}
