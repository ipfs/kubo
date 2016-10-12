package offline

import (
	"context"
	"errors"
	"time"

	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"

	routing "gx/ipfs/QmNUgVQTYnXQVrGT2rajZYsuKV8GYdiL91cdZSQDKNPNgE/go-libp2p-routing"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	cid "gx/ipfs/QmXUuRadqDq5BuFWzVU6VuKaSjTcNm1gNCtLvvP1TJCW4z/go-cid"
	pstore "gx/ipfs/QmXXCcQ7CLg5a81Ui9TTR35QcR4y7ZyihxwfjqaHfUVcVo/go-libp2p-peerstore"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	record "gx/ipfs/QmdM4ohF7cr4MvAECVeD3hRA3HtZrk1ngaek4n8ojVT87h/go-libp2p-record"
	pb "gx/ipfs/QmdM4ohF7cr4MvAECVeD3hRA3HtZrk1ngaek4n8ojVT87h/go-libp2p-record/pb"
	"gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	ci "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"
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

func (c *offlineRouting) PutValue(ctx context.Context, key string, val []byte) error {
	rec, err := record.MakePutRecord(c.sk, key, val, false)
	if err != nil {
		return err
	}
	data, err := proto.Marshal(rec)
	if err != nil {
		return err
	}

	return c.datastore.Put(dshelp.NewKeyFromBinary(key), data)
}

func (c *offlineRouting) GetValue(ctx context.Context, key string) ([]byte, error) {
	v, err := c.datastore.Get(dshelp.NewKeyFromBinary(key))
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
	v, err := c.datastore.Get(dshelp.NewKeyFromBinary(key))
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

func (c *offlineRouting) FindProviders(ctx context.Context, key *cid.Cid) ([]pstore.PeerInfo, error) {
	return nil, ErrOffline
}

func (c *offlineRouting) FindPeer(ctx context.Context, pid peer.ID) (pstore.PeerInfo, error) {
	return pstore.PeerInfo{}, ErrOffline
}

func (c *offlineRouting) FindProvidersAsync(ctx context.Context, k *cid.Cid, max int) <-chan pstore.PeerInfo {
	out := make(chan pstore.PeerInfo)
	close(out)
	return out
}

func (c *offlineRouting) Provide(_ context.Context, k *cid.Cid) error {
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
