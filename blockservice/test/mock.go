package bstest

import (
	. "github.com/ipfs/go-ipfs/blockservice"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap"
	tn "github.com/ipfs/go-ipfs/exchange/bitswap/testnet"
	mockrouting "github.com/ipfs/go-ipfs/routing/mock"
	delay "github.com/ipfs/go-ipfs/thirdparty/delay"
)

type fataler interface {
	Fatal(args ...interface{})
}

// Mocks returns |n| connected mock Blockservices
func Mocks(t fataler, n int) []*BlockService {
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
