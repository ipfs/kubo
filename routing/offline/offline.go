package offline

import (
	"errors"
	"time"

	ds "github.com/ipfs/go-datastore"
	key "github.com/ipfs/go-key"
	record "github.com/libp2p/go-libp2p-record"
	pb "github.com/libp2p/go-libp2p-record/pb"
	routing "github.com/libp2p/go-libp2p-routing"

	proto "github.com/gogo/protobuf/proto"
	ci "github.com/ipfs/go-libp2p-crypto"
	"github.com/ipfs/go-libp2p-peer"
	pstore "github.com/ipfs/go-libp2p-peerstore"
	logging "github.com/ipfs/go-log"
	context "golang.org/x/net/context"
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

func (c *offlineRouting) GetValues(ctx context.Context, key key.Key, _ int) ([]routing.RecvdVal, error) {
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

	return []routing.RecvdVal{
		{Val: rec.GetValue()},
	}, nil
}

func (c *offlineRouting) FindProviders(ctx context.Context, key key.Key) ([]pstore.PeerInfo, error) {
	return nil, ErrOffline
}

func (c *offlineRouting) FindPeer(ctx context.Context, pid peer.ID) (pstore.PeerInfo, error) {
	return pstore.PeerInfo{}, ErrOffline
}

func (c *offlineRouting) FindProvidersAsync(ctx context.Context, k key.Key, max int) <-chan pstore.PeerInfo {
	out := make(chan pstore.PeerInfo)
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
