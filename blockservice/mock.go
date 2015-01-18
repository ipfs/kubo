package blockservice

import (
	"testing"

	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	tn "github.com/jbenet/go-ipfs/exchange/bitswap/testnet"
	mockrouting "github.com/jbenet/go-ipfs/routing/mock"
	delay "github.com/jbenet/go-ipfs/thirdparty/delay"
)

// Mocks returns |n| connected mock Blockservices
func Mocks(t *testing.T, n int) []*BlockService {
	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(0))
	sg := bitswap.NewTestSessionGenerator(net)

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
