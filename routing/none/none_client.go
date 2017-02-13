package nilrouting

import (
	"context"
	"errors"

	repo "github.com/ipfs/go-ipfs/repo"

	pstore "gx/ipfs/QmQMQ2RUjnaEEX8ybmrhuFFGhAwPjyL1Eo6ZoJGD7aAccM/go-libp2p-peerstore"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	cid "gx/ipfs/QmV5gPoRsjN1Gid3LMdNZTyfCtP2DsvqEbMAmz82RmmiGk/go-cid"
	peer "gx/ipfs/QmZcUPvPhD1Xvk6mwijYF8AfR3mG31S1YsEfHG4khrFPRr/go-libp2p-peer"
	routing "gx/ipfs/QmZghcVHwXQC3Zvnvn24LgTmSPkEn2o3PDyKb6nrtPRzRh/go-libp2p-routing"
	p2phost "gx/ipfs/QmbzbRyd22gcW92U1rA2yKagB3myMYhk45XBknJ49F9XWJ/go-libp2p-host"
)

var log = logging.Logger("mockrouter")

type nilclient struct {
}

func (c *nilclient) PutValue(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (c *nilclient) GetValue(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("Tried GetValue from nil routing.")
}

func (c *nilclient) GetValues(_ context.Context, _ string, _ int) ([]routing.RecvdVal, error) {
	return nil, errors.New("Tried GetValues from nil routing.")
}

func (c *nilclient) FindPeer(_ context.Context, _ peer.ID) (pstore.PeerInfo, error) {
	return pstore.PeerInfo{}, nil
}

func (c *nilclient) FindProvidersAsync(_ context.Context, _ *cid.Cid, _ int) <-chan pstore.PeerInfo {
	out := make(chan pstore.PeerInfo)
	defer close(out)
	return out
}

func (c *nilclient) Provide(_ context.Context, _ *cid.Cid) error {
	return nil
}

func (c *nilclient) Bootstrap(_ context.Context) error {
	return nil
}

func ConstructNilRouting(_ context.Context, _ p2phost.Host, _ repo.Datastore) (routing.IpfsRouting, error) {
	return &nilclient{}, nil
}

//  ensure nilclient satisfies interface
var _ routing.IpfsRouting = &nilclient{}
