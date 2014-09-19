package network

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
)

// Forwarder receives messages and forwards them to the delegate.
//
// Forwarder breaks the circular dependency between the BitSwap Session and the
// Network Service.
type Forwarder struct {
	delegate Receiver
}

func (r *Forwarder) ReceiveMessage(
	ctx context.Context, sender *peer.Peer, incoming bsmsg.BitSwapMessage) (
	*peer.Peer, bsmsg.BitSwapMessage, error) {
	if r.delegate == nil {
		return nil, nil, nil
	}
	return r.delegate.ReceiveMessage(ctx, sender, incoming)
}

func (r *Forwarder) Delegate(delegate Receiver) {
	r.delegate = delegate
}
