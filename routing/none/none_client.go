package nilrouting

import (
	"errors"

	key "github.com/ipfs/go-ipfs/blocks/key"
	repo "github.com/ipfs/go-ipfs/repo"
	routing "github.com/ipfs/go-ipfs/routing"
	peer "gx/ipfs/QmQGwpJy9P4yXZySmqkZEXCmbBpJUb8xntCv8Ca4taZwDC/go-libp2p-peer"
	p2phost "gx/ipfs/QmQkQP7WmeT9FRJDsEzAaGYDparttDiB6mCpVBrq2MuWQS/go-libp2p/p2p/host"
	pstore "gx/ipfs/QmXHUpFsnpCmanRnacqYkFoLoFfEq5yS2nUgGkAjJ1Nj9j/go-libp2p-peerstore"
	logging "gx/ipfs/QmYtB7Qge8cJpXc4irsEp8zRqfnZMBeB7aTrMEkPk67DRv/go-log"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var log = logging.Logger("mockrouter")

type nilclient struct {
}

func (c *nilclient) PutValue(_ context.Context, _ key.Key, _ []byte) error {
	return nil
}

func (c *nilclient) GetValue(_ context.Context, _ key.Key) ([]byte, error) {
	return nil, errors.New("Tried GetValue from nil routing.")
}

func (c *nilclient) GetValues(_ context.Context, _ key.Key, _ int) ([]routing.RecvdVal, error) {
	return nil, errors.New("Tried GetValues from nil routing.")
}

func (c *nilclient) FindPeer(_ context.Context, _ peer.ID) (pstore.PeerInfo, error) {
	return pstore.PeerInfo{}, nil
}

func (c *nilclient) FindProvidersAsync(_ context.Context, _ key.Key, _ int) <-chan pstore.PeerInfo {
	out := make(chan pstore.PeerInfo)
	defer close(out)
	return out
}

func (c *nilclient) Provide(_ context.Context, _ key.Key) error {
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
