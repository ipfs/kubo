package offline

import (
	"errors"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	"github.com/jbenet/go-ipfs/p2p/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	record "github.com/jbenet/go-ipfs/routing/record"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	u "github.com/jbenet/go-ipfs/util"
)

var log = eventlog.Logger("offlinerouting")

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

func (c *offlineRouting) PutValue(ctx context.Context, key u.Key, val []byte) error {
	rec, err := record.MakePutRecord(c.sk, key, val)
	if err != nil {
		return err
	}
	data, err := proto.Marshal(rec)
	if err != nil {
		return err
	}

	return c.datastore.Put(key.DsKey(), data)
}

func (c *offlineRouting) GetValue(ctx context.Context, key u.Key) ([]byte, error) {
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

func (c *offlineRouting) FindProviders(ctx context.Context, key u.Key) ([]peer.PeerInfo, error) {
	return nil, ErrOffline
}

func (c *offlineRouting) FindPeer(ctx context.Context, pid peer.ID) (peer.PeerInfo, error) {
	return peer.PeerInfo{}, ErrOffline
}

func (c *offlineRouting) FindProvidersAsync(ctx context.Context, k u.Key, max int) <-chan peer.PeerInfo {
	out := make(chan peer.PeerInfo)
	close(out)
	return out
}

func (c *offlineRouting) Provide(_ context.Context, key u.Key) error {
	return ErrOffline
}

func (c *offlineRouting) Ping(ctx context.Context, p peer.ID) (time.Duration, error) {
	return 0, ErrOffline
}

// ensure offlineRouting matches the IpfsRouting interface
var _ routing.IpfsRouting = &offlineRouting{}
