package mockrouting

import (
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	sync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/sync"
	dht "github.com/ipfs/go-ipfs/routing/dht"
	"github.com/ipfs/go-ipfs/thirdparty/testutil"
	mocknet "gx/ipfs/QmRW2xiYTpDLWTHb822ZYbPBoh3dGLJwaXLGS9tnPyWZpq/go-libp2p/p2p/net/mock"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
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
