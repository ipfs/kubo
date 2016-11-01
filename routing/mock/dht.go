package mockrouting

import (
	context "context"
	"github.com/ipfs/go-ipfs/thirdparty/testutil"
	mocknet "gx/ipfs/QmQ7iWUfqrLEoJwsoNdrZu4625bKyhZCi4Sh6MfjywEfbG/go-libp2p/p2p/net/mock"
	dht "gx/ipfs/QmaWzyiqs7sUayh2G1EaovupWrA1qdKXqRMYA97ruU1xS5/go-libp2p-kad-dht"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	sync "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/sync"
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
