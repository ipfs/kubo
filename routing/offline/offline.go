package offline

import (
	"errors"
	"time"

	proto "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	key "github.com/ipfs/go-ipfs/blocks/key"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	"github.com/ipfs/go-ipfs/p2p/peer"
	routing "github.com/ipfs/go-ipfs/routing"
	pb "github.com/ipfs/go-ipfs/routing/dht/pb"
	record "github.com/ipfs/go-ipfs/routing/record"
	logging "github.com/ipfs/go-ipfs/vendor/go-log-v1.0.0"
)

var log = logging.Logger("offlinerouting")

var ErrOffline = errors.New("routing system in offline mode")

func NewOfflineRouter(dstore ds.Datastore, privkey ci.PrivKey) routing.IpfsRouting {
	return &offlineRouting{
		datastore: dstore,
		sk:        privkey,
	}
}

// offlineRouting implements the IpfsRouting interface,
// but only provides the capability to Put and Get signed dht
// records to and from the local datastore.
type offlineRouting struct {
	datastore ds.Datastore
	sk        ci.PrivKey
}

func (c *offlineRouting) PutValue(ctx context.Context, key key.Key, val []byte) error {
	rec, err := record.MakePutRecord(c.sk, key, val, false)
	if err != nil {
		return err
	}
	data, err := proto.Marshal(rec)
	if err != nil {
		return err
	}

	return c.datastore.Put(key.DsKey(), data)
}

func (c *offlineRouting) GetValue(ctx context.Context, key key.Key) ([]byte, error) {
	v, err := c.datastore.Get(key.DsKey())
	if err != nil {
		return nil, err
	}

	byt, ok := v.([]byte)
	if !ok {
		return nil, errors.New("value stored in datastore not []byte")
	}
	rec := new(pb.Record)
	err = proto.Unmarshal(byt, rec)
	if err != nil {
		return nil, err
	}

	return rec.GetValue(), nil
}

func (c *offlineRouting) FindProviders(ctx context.Context, key key.Key) ([]peer.PeerInfo, error) {
	return nil, ErrOffline
}

func (c *offlineRouting) FindPeer(ctx context.Context, pid peer.ID) (peer.PeerInfo, error) {
	return peer.PeerInfo{}, ErrOffline
}

func (c *offlineRouting) FindProvidersAsync(ctx context.Context, k key.Key, max int) <-chan peer.PeerInfo {
	out := make(chan peer.PeerInfo)
	close(out)
	return out
}

func (c *offlineRouting) Provide(_ context.Context, key key.Key) error {
	return ErrOffline
}

func (c *offlineRouting) Ping(ctx context.Context, p peer.ID) (time.Duration, error) {
	return 0, ErrOffline
}

func (c *offlineRouting) Bootstrap(context.Context) error {
	return nil
}

// ensure offlineRouting matches the IpfsRouting interface
var _ routing.IpfsRouting = &offlineRouting{}
