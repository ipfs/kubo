package mockrouting

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/testutil"
)

var log = u.Logger("mockrouter")

type client struct {
	datastore ds.Datastore
	server    server
	peer      testutil.Identity
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

func (c *client) FindProviders(ctx context.Context, key u.Key) ([]peer.PeerInfo, error) {
	return c.server.Providers(key), nil
}

func (c *client) FindPeer(ctx context.Context, pid peer.ID) (peer.PeerInfo, error) {
	log.Debugf("FindPeer: %s", pid)
	return peer.PeerInfo{}, nil
}

func (c *client) FindProvidersAsync(ctx context.Context, k u.Key, max int) <-chan peer.PeerInfo {
	out := make(chan peer.PeerInfo)
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
	info := peer.PeerInfo{
		ID:    c.peer.ID(),
		Addrs: []ma.Multiaddr{c.peer.Address()},
	}
	return c.server.Announce(info, key)
}

var _ routing.IpfsRouting = &client{}
