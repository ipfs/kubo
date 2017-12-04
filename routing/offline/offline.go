package offline

import (
	"context"
	"errors"
	"time"

	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"

	routing "gx/ipfs/QmPCGUjMRuBcPybZFpjhzpifwPP9wPRoiy5geTQKU4vqWA/go-libp2p-routing"
	record "gx/ipfs/QmWGtsyPYEoiqTtWLpeUA2jpW4YSZgarKDD2zivYAFz7sR/go-libp2p-record"
	pb "gx/ipfs/QmWGtsyPYEoiqTtWLpeUA2jpW4YSZgarKDD2zivYAFz7sR/go-libp2p-record/pb"
	"gx/ipfs/QmWNY7dV54ZDYmTA1ykVdwNCqC11mpU4zSUp6XDpLTH9eG/go-libp2p-peer"
	pstore "gx/ipfs/QmYijbtjCxFEjSXaudaQAUz3LN5VKLssm8WCUsRoqzXmQR/go-libp2p-peerstore"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	ds "gx/ipfs/QmdHG8MAuARdGHxx4rPQASLcvhz24fzjSQq7AJRAQEorq5/go-datastore"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
)

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

func (c *offlineRouting) PutValue(ctx context.Context, key string, val []byte) error {
	rec, err := record.MakePutRecord(c.sk, key, val, false)
	if err != nil {
		return err
	}
	data, err := proto.Marshal(rec)
	if err != nil {
		return err
	}

	return c.datastore.Put(dshelp.NewKeyFromBinary([]byte(key)), data)
}

func (c *offlineRouting) GetValue(ctx context.Context, key string) ([]byte, error) {
	v, err := c.datastore.Get(dshelp.NewKeyFromBinary([]byte(key)))
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

func (c *offlineRouting) GetValues(ctx context.Context, key string, _ int) ([]routing.RecvdVal, error) {
	v, err := c.datastore.Get(dshelp.NewKeyFromBinary([]byte(key)))
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

func (c *offlineRouting) FindPeer(ctx context.Context, pid peer.ID) (pstore.PeerInfo, error) {
	return pstore.PeerInfo{}, ErrOffline
}

func (c *offlineRouting) FindProvidersAsync(ctx context.Context, k *cid.Cid, max int) <-chan pstore.PeerInfo {
	out := make(chan pstore.PeerInfo)
	close(out)
	return out
}

func (c *offlineRouting) Provide(_ context.Context, k *cid.Cid, _ bool) error {
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
