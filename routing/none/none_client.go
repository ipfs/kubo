package nilrouting

import (
	"errors"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	key "github.com/ipfs/go-ipfs/blocks/key"
	p2phost "github.com/ipfs/go-ipfs/p2p/host"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	repo "github.com/ipfs/go-ipfs/repo"
	routing "github.com/ipfs/go-ipfs/routing"
	logging "github.com/ipfs/go-ipfs/vendor/QmQg1J6vikuXF9oDvm4wpdeAUvvkVEKW1EYDw9HhTMnP2b/go-log"
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

func (c *nilclient) FindPeer(_ context.Context, _ peer.ID) (peer.PeerInfo, error) {
	return peer.PeerInfo{}, nil
}

func (c *nilclient) FindProvidersAsync(_ context.Context, _ key.Key, _ int) <-chan peer.PeerInfo {
	out := make(chan peer.PeerInfo)
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
