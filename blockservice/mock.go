package blockservice

import (
	"testing"

	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	tn "github.com/jbenet/go-ipfs/exchange/bitswap/testnet"
	"github.com/jbenet/go-ipfs/routing/mock"
)

// Mocks returns |n| connected mock Blockservices
func Mocks(t *testing.T, n int) []*BlockService {
	net := tn.VirtualNetwork()
	rs := mock.VirtualRoutingServer()
	sg := bitswap.NewSessionGenerator(net, rs)

	instances := sg.Instances(n)

	var servs []*BlockService
	for _, i := range instances {
		bserv, err := New(i.Blockstore(), i.Exchange)
		if err != nil {
			t.Fatal(err)
		}
		servs = append(servs, bserv)
	}
	return servs
}
