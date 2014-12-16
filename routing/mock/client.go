package mockrouting

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("mockrouter")

type client struct {
	datastore ds.Datastore
	server    server
	peer      peer.Peer
}

// FIXME(brian): is this method meant to simulate putting a value into the network?
func (c *client) PutValue(ctx context.Context, key u.Key, val []byte) error {
	log.Debugf("PutValue: %s", key)
	return c.datastore.Put(key.DsKey(), val)
}

// FIXME(brian): is this method meant to simulate getting a value from the network?
func (c *client) GetValue(ctx context.Context, key u.Key) ([]byte, error) {
	log.Debugf("GetValue: %s", key)
	v, err := c.datastore.Get(key.DsKey())
	if err != nil {
		return nil, err
	}

	data, ok := v.([]byte)
	if !ok {
		return nil, errors.New("could not cast value from datastore")
	}

	return data, nil
}

func (c *client) FindProviders(ctx context.Context, key u.Key) ([]peer.Peer, error) {
	return c.server.Providers(key), nil
}

func (c *client) FindPeer(ctx context.Context, pid peer.ID) (peer.Peer, error) {
	log.Debugf("FindPeer: %s", pid)
	return nil, nil
}

func (c *client) FindProvidersAsync(ctx context.Context, k u.Key, max int) <-chan peer.Peer {
	out := make(chan peer.Peer)
	go func() {
		defer close(out)
		for i, p := range c.server.Providers(k) {
			if max <= i {
				return
			}
			select {
			case out <- p:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// Provide returns once the message is on the network. Value is not necessarily
// visible yet.
func (c *client) Provide(_ context.Context, key u.Key) error {
	return c.server.Announce(c.peer, key)
}

var _ routing.IpfsRouting = &client{}
