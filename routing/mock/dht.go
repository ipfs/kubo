package mockrouting

import (
	context "context"
	mocknet "gx/ipfs/QmRLAc3sN9cTLDpNYkz9dGoGc9wXY1svkJY2RrVbuYcD4V/go-libp2p/p2p/net/mock"
	"gx/ipfs/QmUPhGaiqwLkwqLTa1QqgD9jwDkuMRSPdkgNhXuKBWiH7R/go-testutil"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	sync "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore/sync"
	dht "gx/ipfs/QmfPWAEd2CPb6chn16EFBREVXG8dtLkaPEA8ZF9Mza2jYs/go-libp2p-kad-dht"
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
