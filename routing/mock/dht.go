package mockrouting

import (
	context "context"
	"github.com/ipfs/go-ipfs/thirdparty/testutil"
	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	sync "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/sync"
	mocknet "gx/ipfs/QmSNJRX4uphb3Eyp69uYbpRVvgqjPxfjnJmjcdMWkDH5Pn/go-libp2p/p2p/net/mock"
	dht "gx/ipfs/QmdFu71pRmWMNWht96ZTJ3wRx4D7BPJ2JfHH24z59Gidsc/go-libp2p-kad-dht"
)

type mocknetserver struct {
	mn mocknet.Mocknet
}

func NewDHTNetwork(mn mocknet.Mocknet) Server {
	return &mocknetserver{
		mn: mn,
	}
}

func (rs *mocknetserver) Client(p testutil.Identity) Client {
	return rs.ClientWithDatastore(context.TODO(), p, ds.NewMapDatastore())
}

func (rs *mocknetserver) ClientWithDatastore(ctx context.Context, p testutil.Identity, ds ds.Datastore) Client {

	// FIXME AddPeer doesn't appear to be idempotent

	host, err := rs.mn.AddPeer(p.PrivateKey(), p.Address())
	if err != nil {
		panic("FIXME")
	}
	return dht.NewDHT(ctx, host, sync.MutexWrap(ds))
}

var _ Server = &mocknetserver{}
