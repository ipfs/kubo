package mockrouting

import (
	context "context"
	dht "gx/ipfs/QmSFNexSqAnJKGfDwKnGx2sGB1EtLwsNQ586u18UkVLjAy/go-libp2p-kad-dht"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	sync "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore/sync"
	mocknet "gx/ipfs/QmXZ6XetFwaDNmszPCux9DaKqMykEJGDtWHSqprn94UXzM/go-libp2p/p2p/net/mock"
	"gx/ipfs/QmZJD56ZWLViJAVkvLc7xbbDerHzUMLr2X4fLRYfbxZWDN/go-testutil"
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
