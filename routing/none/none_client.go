package nilrouting

import (
	"errors"

	repo "github.com/ipfs/go-ipfs/repo"
	key "github.com/ipfs/go-key"
	peer "github.com/ipfs/go-libp2p-peer"
	pstore "github.com/ipfs/go-libp2p-peerstore"
	logging "github.com/ipfs/go-log"
	routing "github.com/libp2p/go-libp2p-routing"
	p2phost "github.com/libp2p/go-libp2p/p2p/host"
	context "golang.org/x/net/context"
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
