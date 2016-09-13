package mockrouting

import (
	"errors"
	"time"

	"github.com/ipfs/go-ipfs/thirdparty/testutil"

	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	peer "gx/ipfs/QmWXjJo15p4pzT7cayEwZi2sWgJqLnGDof6ZGMh9xBgU1p/go-libp2p-peer"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
	routing "gx/ipfs/QmcoQiBzRaaVv1DZbbXoDWiEtvDN94Ca1DcwnQKK2tP92s/go-libp2p-routing"
	pstore "gx/ipfs/QmdMfSLMDBDYhtc4oF3NYGCZr5dy4wQb6Ji26N4D4mdxa2/go-libp2p-peerstore"
	dhtpb "gx/ipfs/Qme7D9iKHYxwq28p6PzCymywsYSRBx9uyGzW7qNB3s9VbC/go-libp2p-record/pb"
)

var log = logging.Logger("mockrouter")

type client struct {
	datastore ds.Datastore
	server    server
	peer      testutil.Identity
}

// FIXME(brian): is this method meant to simulate putting a value into the network?
func (c *client) PutValue(ctx context.Context, key key.Key, val []byte) error {
	log.Debugf("PutValue: %s", key)
	rec := new(dhtpb.Record)
	rec.Value = val
	rec.Key = proto.String(string(key))
	rec.TimeReceived = proto.String(u.FormatRFC3339(time.Now()))
	data, err := proto.Marshal(rec)
	if err != nil {
		return err
	}

	return c.datastore.Put(key.DsKey(), data)
}

// FIXME(brian): is this method meant to simulate getting a value from the network?
func (c *client) GetValue(ctx context.Context, key key.Key) ([]byte, error) {
	log.Debugf("GetValue: %s", key)
	v, err := c.datastore.Get(key.DsKey())
	if err != nil {
		return nil, err
	}

	data, ok := v.([]byte)
	if !ok {
		return nil, errors.New("could not cast value from datastore")
	}

	rec := new(dhtpb.Record)
	err = proto.Unmarshal(data, rec)
	if err != nil {
		return nil, err
	}

	return rec.GetValue(), nil
}

func (c *client) GetValues(ctx context.Context, key key.Key, count int) ([]routing.RecvdVal, error) {
	log.Debugf("GetValues: %s", key)
	data, err := c.GetValue(ctx, key)
	if err != nil {
		return nil, err
	}

	return []routing.RecvdVal{{Val: data, From: c.peer.ID()}}, nil
}

func (c *client) FindProviders(ctx context.Context, key key.Key) ([]pstore.PeerInfo, error) {
	return c.server.Providers(key), nil
}

func (c *client) FindPeer(ctx context.Context, pid peer.ID) (pstore.PeerInfo, error) {
	log.Debugf("FindPeer: %s", pid)
	return pstore.PeerInfo{}, nil
}

func (c *client) FindProvidersAsync(ctx context.Context, k key.Key, max int) <-chan pstore.PeerInfo {
	out := make(chan pstore.PeerInfo)
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
func (c *client) Provide(_ context.Context, key key.Key) error {
	info := pstore.PeerInfo{
		ID:    c.peer.ID(),
		Addrs: []ma.Multiaddr{c.peer.Address()},
	}
	return c.server.Announce(info, key)
}

func (c *client) Ping(ctx context.Context, p peer.ID) (time.Duration, error) {
	return 0, nil
}

func (c *client) Bootstrap(context.Context) error {
	return nil
}

var _ routing.IpfsRouting = &client{}
