package bstest

import (
	. "github.com/ipfs/go-ipfs/blockservice"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap"
	tn "github.com/ipfs/go-ipfs/exchange/bitswap/testnet"

	mockrouting "gx/ipfs/QmQUPmFYZBSWn4mtX1YwYkSaMoWVore7tCiSetr6k8JW21/go-ipfs-routing/mock"
	delay "gx/ipfs/QmRJVNatYJwTAHgdSM1Xef9QVQ1Ch3XHdmcrykjP5Y4soL/go-ipfs-delay"
)

// Mocks returns |n| connected mock Blockservices
func Mocks(n int) []BlockService {
	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(0))
	sg := bitswap.NewTestSessionGenerator(net)

	instances := sg.Instances(n)

	var servs []BlockService
	for _, i := range instances {
		servs = append(servs, New(i.Blockstore(), i.Exchange))
	}
	return servs
}
